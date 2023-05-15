package network

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/madkins23/lsp-tester/tester/flags"
)

type Listener struct {
	flags    *flags.Set
	listener net.Listener
	waiter   *sync.WaitGroup
}

func NewListener(flags *flags.Set, waiter *sync.WaitGroup) (*Listener, error) {
	listener := &Listener{
		flags:  flags,
		waiter: waiter,
	}
	var err error
	if listener.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", flags.ClientPort())); err != nil {
		return nil, fmt.Errorf("open listener connection: %w", err)
	}
	return listener, nil
}

func (l *Listener) ListenForClient(configureFn func(conn net.Conn)) {
	log.Info().Uint("port", l.flags.ClientPort()).Msg("Listener starting")
	defer log.Info().Uint("port", l.flags.ClientPort()).Msg("Listener finished")

	l.waiter.Add(1)
	defer l.waiter.Done()

	for {
		if conn, err := l.listener.Accept(); err == nil {
			configureFn(conn)
		} else if errors.Is(err, net.ErrClosed) {
			break
		} else {
			log.Warn().Err(err).Msg("Listener accept")
		}
	}
}

func (l *Listener) Close() {
	if l.listener != nil {
		_ = l.listener.Close()
	}
}
