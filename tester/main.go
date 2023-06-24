package main

import (
	"net"
	"os"
	"runtime/debug"
	"sync"

	"github.com/madkins23/go-utils/flag"
	"github.com/rs/zerolog/log"

	"github.com/madkins23/lsp-tester/tester/command"
	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/logging"
	"github.com/madkins23/lsp-tester/tester/lsp"
	"github.com/madkins23/lsp-tester/tester/message"
	"github.com/madkins23/lsp-tester/tester/network"
	"github.com/madkins23/lsp-tester/tester/web"
)

func main() {
	var (
		err       error
		flagSet   *flags.Set
		listener  *network.Listener
		logMgr    *logging.Manager
		msgLogger *message.Logger
		waiter    sync.WaitGroup
	)

	flagSet = flags.NewSet()
	if err = flag.LoadSettings(flagSet.FlagSet); err != nil {
		log.Error().Err(err).Msg("Error loading settings file")
		return
	}
	if err = flagSet.Parse(os.Args[1:]); err != nil {
		// Usage will have been done automagically.
		return
	}
	if logMgr, err = logging.NewManager(flagSet); err != nil {
		log.Error().Err(err).Msg("Failed to configure log options")
	}
	defer logMgr.Close()

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")
	if flagSet.Version() {
		logVersion()
	}

	msgLogger = message.NewLogger(flagSet, logMgr)

	if flagSet.HasCommand() {
		if flagSet.ClientPort() != 0 {
			log.Warn().Msg("--process set, --clientPort will be ignored")
		}
		if flagSet.ServerPort() != 0 {
			log.Warn().Msg("--process set, --serverPort will be ignored")
		}

		if process, err := command.NewProcess("server", flagSet, msgLogger, &waiter); err != nil {
			log.Error().Err(err).Msg("Unable to create Process receiver")
			return
		} else if err = process.Start(); err != nil {
			log.Error().Err(err).Msg("Unable to start Process receiver")
			return
		} else {
			sendRequest(flagSet, process, msgLogger)
		}
	} else {
		var client lsp.Receiver
		var server lsp.Receiver

		if flagSet.ServerPort() > 0 {
			connection, err := network.ConnectToLSP(flagSet)
			if err != nil {
				log.Error().Err(err).Msgf("Connect to LSP at %s:%d", flagSet.HostAddress(), flagSet.ServerPort())
				return
			}

			client = network.NewReceiver("server", flagSet, connection, msgLogger, &waiter)
			if err = client.Start(); err != nil {
				log.Error().Err(err).Msg("Unable to start ReceiverBase")
				return
			}

			sendRequest(flagSet, client, msgLogger)
		}

		if flagSet.ClientPort() > 0 {
			if listener, err = network.NewListener(flagSet, &waiter); err != nil {
				log.Error().Err(err).Msgf("Make listener on %d", flagSet.ClientPort())
			} else {
				go listener.ListenForClient(func(conn net.Conn) {
					log.Info().Msg("Accepting client")
					server = network.NewReceiver("client", flagSet, conn, msgLogger, &waiter)
					if err = server.Start(); err != nil {
						log.Error().Err(err).Msg("Unable to start ReceiverBase")
						return
					}
					if client != nil {
						log.Info().Msg("Configuring pass-through operation")
						client.SetOther(server)
						server.SetOther(client)
					}
				})
			}
		}
	}

	webSrvr := web.NewWebServer(flagSet, listener, logMgr, msgLogger, &waiter)
	if flagSet.WebPort() > 0 {
		go webSrvr.Serve()
	}

	waiter.Wait()
}

func logVersion() {
	if info, ok := debug.ReadBuildInfo(); ok {
		var target, arch string
		event := log.Info().Str("Go", info.GoVersion)
		for _, setting := range info.Settings {
			switch setting.Key {
			case "GOOS":
				target = setting.Value
			case "GOARCH":
				arch = setting.Value
			case "vcs":
				event.Str("VCS", setting.Value)
			case "vcs.revision":
				event.Str("Revision", setting.Value)
			case "vcs.time":
				event.Str("Revised", setting.Value)
			case "vcs.modified":
				event.Str("Modified", setting.Value)
			}
		}
		if target != "" && arch != "" {
			target += " " + arch
		} else if target == "" && arch != "" {
			target = arch
		}
		if target != "" {
			event.Str("Target", target)
		}
		event.Str("Main", info.Main.Version)
		event.Msg("Version")
	}
}

func sendRequest(flags *flags.Set, receiver lsp.Receiver, msgLgr *message.Logger) {
	if flags.RequestPath() != "" {
		if rqst, err := message.LoadMessage(flags.RequestPath()); err != nil {
			log.Error().Err(err).Msgf("Load request from file %s", flags.RequestPath())
		} else if err := receiver.SendMessage("server", rqst, msgLgr); err != nil {
			log.Error().Err(err).Msgf("Send message from file %s", flags.RequestPath())
		}
	}
}
