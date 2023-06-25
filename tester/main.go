package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/madkins23/go-utils/flag"
	utilLog "github.com/madkins23/go-utils/log"

	"github.com/madkins23/lsp-tester/tester/protocol/lsp"
	"github.com/madkins23/lsp-tester/tester/protocol/sub"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/logging"
	"github.com/madkins23/lsp-tester/tester/message"
	"github.com/madkins23/lsp-tester/tester/protocol/tcp"
	"github.com/madkins23/lsp-tester/tester/web"
)

func main() {
	var (
		err     error
		flagSet *flags.Set
		logMgr  *logging.Manager
		msgLgr  *message.Logger
		waiter  sync.WaitGroup
	)

	utilLog.Console()

	flagSet = flags.NewSet()
	if err = flag.LoadSettings(flagSet.FlagSet); err != nil {
		log.Error().Err(err).Msg("Error loading settings file")
		return
	}
	if err = flagSet.Parse(os.Args[1:]); err == nil {
	} else if errors.Is(err, flags.ErrHelp) {
		return
	}
	if err = flagSet.ValidateLogging(); err != nil {
		log.Error().Err(err).Msg("Validate logging flags")
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
	if err = flagSet.Validate(); err != nil {
		// Don't bother with error message if -version was used.
		if !flagSet.Version() {
			log.Error().Err(err).Msg("Validate flags")
		}
		return
	}

	msgLgr = message.NewLogger(flagSet, logMgr)

	var listener *tcp.Listener
	switch flagSet.Protocol() {
	case flags.Sub:
		err = commandProtocol(flagSet, msgLgr, &waiter)
	case flags.TCP:
		listener, err = tcpProtocol(flagSet, msgLgr, &waiter)
	default:
		log.Error().Str("protocol", flagSet.Protocol().String()).Msg("Unknown LSP communication protocol")
		return
	}
	if err != nil {
		log.Error().Err(err).Str("protocol", flagSet.Protocol().String()).Msg("LSP setup error")
		return
	}

	webSrvr := web.NewWebServer(flagSet, listener, logMgr, msgLgr, &waiter)
	if flagSet.WebPort() > 0 {
		go webSrvr.Serve()
	}

	waiter.Wait()
}

func commandProtocol(flagSet *flags.Set, msgLogger *message.Logger, waiter *sync.WaitGroup) error {
	var err error
	var process lsp.Receiver
	if flagSet.ModeConnectsToServer() {
		process, err = sub.NewProcess("server", flagSet, msgLogger, waiter)
		if err != nil {
			return fmt.Errorf("create Process receiver: %w", err)
		} else if err = process.Start(); err != nil {
			return fmt.Errorf("start Process receiver: %w", err)
		} else {
			sendRequest(flagSet, process, msgLogger)
		}
	}

	if flagSet.ModeConnectsToClient() {
		caller := sub.NewCaller("client", flagSet, msgLogger, waiter)
		if err := caller.Start(); err != nil {
			return fmt.Errorf("start Caller receiver: %w", err)
		}
		if process != nil {
			caller.SetOther(process)
			process.SetOther(caller)
		}
	}

	return nil
}

func tcpProtocol(flagSet *flags.Set, msgLogger *message.Logger, waiter *sync.WaitGroup) (*tcp.Listener, error) {
	var client lsp.Receiver
	if flagSet.ModeConnectsToServer() {
		connection, err := tcp.ConnectToLSP(flagSet)
		if err != nil {
			return nil, fmt.Errorf("connect to LSP %s:%d: %w", flagSet.HostAddress(), flagSet.ServerPort(), err)
		}

		client = tcp.NewReceiver("server", flagSet, connection, msgLogger, waiter)
		if err = client.Start(); err != nil {
			return nil, fmt.Errorf("create server Receiver: %w", err)
		}

		sendRequest(flagSet, client, msgLogger)
	}

	var err error
	var listener *tcp.Listener
	if flagSet.ModeConnectsToClient() {
		if listener, err = tcp.NewListener(flagSet, waiter); err != nil {
			log.Error().Err(err).Msgf("Make listener on %d", flagSet.ClientPort())
		} else {
			var server lsp.Receiver
			ready := make(chan bool)
			go listener.ListenForClient(ready, func(conn net.Conn) {
				log.Info().Msg("Accepting client")
				server = tcp.NewReceiver("client", flagSet, conn, msgLogger, waiter)
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
			<-ready // Wait for listener to add to waiter.
		}
	}

	return listener, nil
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
