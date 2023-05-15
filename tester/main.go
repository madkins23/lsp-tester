package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/madkins23/go-utils/log"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/logging"
)

var flagSet = flags.NewSet()
var logMgr *logging.Manager

var (
	listener net.Listener
	waiter   sync.WaitGroup
)

func main() {
	if err := flagSet.Parse(os.Args[1:]); err != nil {
		// Usage will have been done automagically.
		return
	}

	var err error
	if logMgr, err = logging.NewManager(flagSet); err != nil {
		log.Error().Err(err).Msg("Failed to configure log options")
	}
	defer logMgr.Close()

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	var client *receiver
	var server *receiver

	if flagSet.ServerPort() > 0 {
		connection, err := connectToLSP(flagSet.HostAddress(), flagSet.ServerPort())
		if err != nil {
			log.Error().Err(err).Msgf("Connect to LSP at %s:%d", flagSet.HostAddress(), flagSet.ServerPort())
			return
		}

		if client, err = startReceiver("server", connection); err != nil {
			log.Error().Err(err).Msg("Unable to start receiver")
			return
		}

		if flagSet.RequestPath() != "" {
			if rqst, err := loadMessage(flagSet.RequestPath()); err != nil {
				log.Error().Err(err).Msgf("Load request from file %s", flagSet.RequestPath())
			} else if err := sendMessage("server", rqst, connection); err != nil {
				log.Error().Err(err).Msgf("Send message from file %s", flagSet.RequestPath())
			}
		}
	}

	if flagSet.ClientPort() > 0 {
		var err error
		if listener, err = net.Listen("tcp", fmt.Sprintf(":%d", flagSet.ClientPort())); err != nil {
			log.Error().Err(err).Msgf("Make listener on %d", flagSet.ClientPort())
		} else {
			go listenForClient(flagSet.ClientPort(), listener, func(conn net.Conn) {
				log.Info().Msg("Accepting client")
				if server, err = startReceiver("client", conn); err != nil {
					log.Error().Err(err).Msg("Unable to start receiver")
					return
				}
				if client != nil {
					log.Info().Msg("Configuring pass-through operation")
					client.other = server
					server.other = client
				}
			})
		}
	}

	if flagSet.WebPort() > 0 {
		go webServer()
	}

	waiter.Wait()
}

func connectToLSP(host string, port uint) (net.Conn, error) {
	tcpAddress := host + ":" + strconv.Itoa(int(port))
	var connection *net.TCPConn

	if tcpAddr, err := net.ResolveTCPAddr("tcp", tcpAddress); err != nil {
		return nil, fmt.Errorf("resolve TCP address: %w", err)
	} else if connection, err = net.DialTCP("tcp", nil, tcpAddr); err != nil {
		return nil, fmt.Errorf("dial TCP address: %w", err)
	} else {
		return connection, nil
	}
}

func listenForClient(port uint, listener net.Listener, configureFn func(conn net.Conn)) {
	log.Info().Uint("port", port).Msg("Listener starting")
	defer log.Info().Uint("port", port).Msg("Listener finished")

	waiter.Add(1)
	defer waiter.Done()

	for {
		if conn, err := listener.Accept(); err == nil {
			configureFn(conn)
		} else if errors.Is(err, net.ErrClosed) {
			break
		} else {
			log.Warn().Err(err).Msg("Listener accept")
		}
	}
}
