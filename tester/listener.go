package main

import (
	"fmt"
	"net"

	"github.com/madkins23/go-utils/log"
)

func listenForClient(port uint) (net.Conn, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Error().Err(err).Msg("Closing listener")
		}
	}()

	if conn, err := listener.Accept(); err != nil {
		return nil, fmt.Errorf("accept conn: %w", err)
	} else {
		return conn, nil
	}
}
