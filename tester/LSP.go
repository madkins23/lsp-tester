package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/madkins23/go-utils/log"
)

const (
	JSON_RPC = "2.0"
)

func main() {
	var ()

	var (
		hostAddress string
		clientPort  uint
		serverPort  uint
		requestPath string
		expandJSON  bool
	)

	flags := flag.NewFlagSet("LSP", flag.ContinueOnError)
	flags.StringVar(&hostAddress, "host", "127.0.0.1", "Host address")
	flags.StringVar(&requestPath, "request", "", "Path to requestPath file (client mode)")
	flags.UintVar(&clientPort, "clientPort", 0, "Client port number")
	flags.UintVar(&serverPort, "serverPort", 0, "Server port number")
	flags.BoolVar(&expandJSON, "expand", false, "Expand message JSON in log if true")

	logConfig := log.ConsoleOrFile{}
	logConfig.AddFlagsToSet(flags, "/tmp/LSP.log")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Printf("ResponseError parsing command line flags: %s.", err)
			flags.Usage()
		}
		return
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
			fixed.FormatExtra = extraJson
			log.SetLogger(log.Logger().Output(fixed))
		}
	}

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	if clientPort > 0 {
		go func() {
			if err := client(hostAddress, clientPort, requestPath); err != nil {
				log.Error().Err(err).Msg("ResponseError in client.")
			}
		}()
	}

	time.Sleep(time.Hour)
}

func server(port uint) error {
	log.Info().Msg("LSP server mode.")

	return nil
}
