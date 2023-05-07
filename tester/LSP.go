package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/madkins23/go-utils/log"
)

var (
	simpleFmt bool
)

func main() {
	var (
		hostAddress string
		clientPort  uint
		serverPort  uint
		requestPath string
		expandJSON  bool
	)

	flags := flag.NewFlagSet("lsp-tester", flag.ContinueOnError)
	flags.StringVar(&hostAddress, "host", "127.0.0.1", "Host address")
	flags.StringVar(&requestPath, "request", "", "Path to requestPath file (client mode)")
	flags.UintVar(&clientPort, "clientPort", 0, "Client port number")
	flags.UintVar(&serverPort, "serverPort", 0, "Server port number")
	flags.BoolVar(&expandJSON, "expand", false, "Expand message JSON in log")
	flags.BoolVar(&simpleFmt, "simple", false, "Simple message format")

	logConfig := log.ConsoleOrFile{}
	logConfig.AddFlagsToSet(flags, "/tmp/lsp-tester.log") // what if we're on Windows?

	if err := flags.Parse(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Printf("Error parsing command line flags: %s.", err)
			flags.Usage()
		}
		return
	}

	if expandJSON && simpleFmt {
		fmt.Println("Flags -expand and -simple are mutually exclusive.")
		flags.Usage()
	}

	if err := logConfig.Setup(); err != nil {
		fmt.Printf("Log file creation error: %s.", err)
		return
	}
	defer logConfig.CloseForDefer()

	if expandJSON {
		if writer := logConfig.Writer(); writer != nil {
			fixed := *writer
			fixed.FieldsExclude = []string{"msg"}
			fixed.FormatExtra = formatMessageJSON
			log.SetLogger(log.Logger().Output(fixed))
		}
	}

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	var client *receiver
	var server *receiver

	if clientPort > 0 {
		if connection, err := connectToLSP(hostAddress, clientPort); err != nil {
			log.Error().Err(err).Msgf("Connect to LSP at %s:%d", hostAddress, clientPort)
			return
		} else {
			client = newReceiver("client", "server", connection)
			go client.receive()

			if requestPath != "" {
				loadLog := log.Logger().With().Str("source", "file").Logger()
				if rqst, err := loadRequest(requestPath); err != nil {
					log.Error().Err(err).Msgf("Load request from file %s", requestPath)
				} else if err := sendRequest("server", rqst, connection, &loadLog); err != nil {
					log.Error().Err(err).Msgf("Send message from file %s", requestPath)
				}
			}
		}
	}

	if serverPort > 0 {
		err := listenForClient(serverPort, func(conn net.Conn) error {
			log.Info().Msg("Accepting client connection")
			server = newReceiver("server", "client", conn)
			if client != nil {
				log.Info().Msg("Configuring pass-through operation")
				client.other = server
				server.other = client
			}
			go server.receive()
			return nil
		})
		if err != nil {
			log.Error().Err(err).Msgf("Listen as LSP on port %d", serverPort)
			return
		}
	}

	time.Sleep(time.Hour)
}

func listenForClient(port uint, configureFn func(conn net.Conn) error) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Error().Err(err).Msg("Closing listener")
		}
	}()

	for {
		if conn, err := listener.Accept(); err != nil {
			log.Warn().Err(err).Msg("Accept connection")
		} else if err = configureFn(conn); err != nil {
			log.Warn().Err(err).Msg("Configuring ")
		}
	}
}
