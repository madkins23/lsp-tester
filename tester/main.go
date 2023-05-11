package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

var (
	simpleFormat bool
	messageDir   string
)

var (
	defaultWriter  *zerolog.ConsoleWriter
	expandedWriter *zerolog.ConsoleWriter
	logFormat      = "default"
)

var (
	listener  net.Listener
	messages  []string
	receivers = make(map[string]*receiver)
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
	flags.StringVar(&messageDir, "messages", "", "Path to directory of message files")

	logConfig := log.ConsoleOrFile{}
	logConfig.AddFlagsToSet(flags, path.Join(os.TempDir(), "/lsp-tester.log"))

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
	} else if simpleFormat {
		logFormat = "simple"
	}

	if err := logConfig.Setup(); err != nil {
		fmt.Printf("Log file creation error: %s.", err)
		return
	}
	defer logConfig.CloseForDefer()

	if writer := logConfig.Writer(); writer != nil {
		// Setup variants of ConsoleWriter so that the logger con be change at runtime.
		defaultWriter = logConfig.Writer()
		expanded := *defaultWriter
		expanded.FieldsExclude = []string{"msg"}
		expanded.FormatExtra = formatMessageJSON
		expandedWriter = &expanded
		if expandJSON {
			log.SetLogger(log.Logger().Output(expandedWriter))
			logFormat = "expanded"
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

		if client, err = startReceiver("server", connection); err != nil {
			log.Error().Err(err).Msg("Unable to start receiver")
			return
		}

		if requestPath != "" {
			if rqst, err := loadRequest(requestPath); err != nil {
				log.Error().Err(err).Msgf("Load request from file %s", requestPath)
			} else if err := sendRequest("server", rqst, connection); err != nil {
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
