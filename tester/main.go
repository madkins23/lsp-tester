package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

var (
	defaultWriter  *zerolog.ConsoleWriter
	expandedWriter *zerolog.ConsoleWriter
	simpleFormat   bool
	listener       net.Listener
	receivers      = make(map[string]*receiver)
	// TODO: Make something that can return number of waiting things.
	waiter sync.WaitGroup
)

func main() {
	var (
		hostAddress string
		clientPort  uint
		serverPort  uint
		requestPath string
		expandJSON  bool
		webPort     uint
	)

	flags := flag.NewFlagSet("lsp-tester", flag.ContinueOnError)
	flags.StringVar(&hostAddress, "host", "127.0.0.1", "Host address")
	flags.StringVar(&requestPath, "request", "", "Path to requestPath file (client mode)")
	flags.UintVar(&clientPort, "clientPort", 0, "Port number for contacting LSP as client")
	flags.UintVar(&serverPort, "serverPort", 0, "Port number served for extension to contact")
	flags.BoolVar(&expandJSON, "expand", false, "Expand message JSON in log")
	flags.BoolVar(&simpleFormat, "simple", false, "Simple message format")
	flags.UintVar(&webPort, "webPort", 0, "Web port number to enable web access")

	logConfig := log.ConsoleOrFile{}
	logConfig.AddFlagsToSet(flags, "/tmp/lsp-tester.log") // what if we're on Windows?

	if err := flags.Parse(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Printf("Error parsing command line flags: %s.", err)
			flags.Usage()
		}
		return
	}

	if expandJSON && simpleFormat {
		fmt.Println("Flags -expand and -simple are mutually exclusive.")
		flags.Usage()
	}

	if err := logConfig.Setup(); err != nil {
		fmt.Printf("Log file creation error: %s.", err)
		return
	}
	defer logConfig.CloseForDefer()

	if writer := logConfig.Writer(); writer != nil {
		// Setup variants of ConsoleWriter so that the logger con be change at runtime.
		defaultWriter = logConfig.Writer()
		expand := *defaultWriter
		expand.FieldsExclude = []string{"msg"}
		expand.FormatExtra = formatMessageJSON
		expandedWriter = &expand
		if expandJSON {
			log.SetLogger(log.Logger().Output(expandedWriter))
		}
	}

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	var client *receiver
	var server *receiver

	if clientPort > 0 {
		connection, err := connectToLSP(hostAddress, clientPort)
		if err != nil {
			log.Error().Err(err).Msgf("Connect to LSP at %s:%d", hostAddress, clientPort)
			return
		}

		client = newReceiver("server", connection)
		go client.receive()

		if requestPath != "" {
			loadLog := log.Logger().With().Str("src", "file").Logger()
			if rqst, err := loadRequest(requestPath); err != nil {
				log.Error().Err(err).Msgf("Load request from file %s", requestPath)
			} else if err := sendRequest("server", rqst, connection, &loadLog); err != nil {
				log.Error().Err(err).Msgf("Send message from file %s", requestPath)
			}
		}
	}

	if serverPort > 0 {
		var err error
		if listener, err = net.Listen("tcp", fmt.Sprintf(":%d", serverPort)); err != nil {
			log.Error().Err(err).Msgf("Make listener on %d", serverPort)
		} else {
			go listenForClient(serverPort, listener, func(conn net.Conn) {
				log.Info().Msg("Accepting client connectedTo")
				server = newReceiver("client", conn)
				if client != nil {
					log.Info().Msg("Configuring pass-through operation")
					client.other = server
					server.other = client
				}
				go server.receive()
			})
		}
	}

	if webPort > 0 {
		go webServer(webPort)
	}

	// TODO: something less arbitrary here
	time.Sleep(1 * time.Second)

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
	defer func() {
		waiter.Done()
	}()

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
