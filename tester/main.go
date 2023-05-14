package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/madkins23/go-utils/log"
)

var (
	listener net.Listener
	waiter   sync.WaitGroup
)

func main() {
	var (
		hostAddress string
		clientPort  uint
		serverPort  uint
		requestPath string
		webPort     uint
	)

	flags := flag.NewFlagSet("lsp-tester", flag.ContinueOnError)
	flags.StringVar(&hostAddress, "host", "127.0.0.1", "Host address")
	flags.UintVar(&clientPort, "clientPort", 0, "Port number served for extension to contact")
	flags.UintVar(&serverPort, "serverPort", 0, "Port number on which to contact LSP server")
	flags.UintVar(&webPort, "webPort", 0, "Web port number to enable web access")
	flags.StringVar(&messageDir, "messages", "", "Path to directory of message files")
	flags.StringVar(&requestPath, "request", "", "Path to requestPath file (client mode)")
	flags.StringVar(&logLevel, "logLevel", "info", "Set log level")
	flags.StringVar(&stdFormat, "logFormat", fmtDefault, "Console output format")
	flags.StringVar(&logFilePath, "logFile", "", "Log file path")
	flags.BoolVar(&fileAppend, "fileAppend", false, "Append to any pre-existing log file")
	flags.StringVar(&fileFormat, "fileFormat", fmtDefault, "Log file format")

	if err := flags.Parse(os.Args[1:]); err != nil {
		return
	}

	if messageDir != "" {
		if strings.HasPrefix(messageDir, "~/") {
			dirname, _ := os.UserHomeDir()
			messageDir = filepath.Join(dirname, messageDir[2:])
		}
		if stat, err := os.Stat(messageDir); err != nil {
			log.Error().Str("-messages", messageDir).Msg("Unable to verify existence of message directory")
		} else if !stat.IsDir() {
			log.Error().Str("-messages", messageDir).Msg("Not a directory")
		}
	}

	var formatFailure bool
	if !isFormat[stdFormat] {
		log.Error().Str("-stdFormat", stdFormat).Msg("Unrecognized stdFormat")
		formatFailure = true
	}
	if logFilePath != "" && !isFormat[fileFormat] {
		log.Error().Str("-fileFormat", stdFormat).Msg("Unrecognized log file format")
		formatFailure = true
	}
	if formatFailure {
		return
	}

	if err := logSetup(); err != nil {
		log.Error().Err(err).Msg("Failed to configure log options")
		return
	}
	defer logShutdown()
	setStdFormat()
	setFileFormat()

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	var client *receiver
	var server *receiver

	if serverPort > 0 {
		connection, err := connectToLSP(hostAddress, serverPort)
		if err != nil {
			log.Error().Err(err).Msgf("Connect to LSP at %s:%d", hostAddress, serverPort)
			return
		}

		if client, err = startReceiver("server", connection); err != nil {
			log.Error().Err(err).Msg("Unable to start receiver")
			return
		}

		if requestPath != "" {
			if strings.HasPrefix(requestPath, "~/") {
				dirname, _ := os.UserHomeDir()
				requestPath = filepath.Join(dirname, requestPath[2:])
			}
			if rqst, err := loadMessage(requestPath); err != nil {
				log.Error().Err(err).Msgf("Load request from file %s", requestPath)
			} else if err := sendMessage("server", rqst, connection); err != nil {
				log.Error().Err(err).Msgf("Send message from file %s", requestPath)
			}
		}
	}

	if clientPort > 0 {
		var err error
		if listener, err = net.Listen("tcp", fmt.Sprintf(":%d", clientPort)); err != nil {
			log.Error().Err(err).Msgf("Make listener on %d", clientPort)
		} else {
			go listenForClient(clientPort, listener, func(conn net.Conn) {
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

	if webPort > 0 {
		go webServer(webPort)
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
