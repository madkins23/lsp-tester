package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

const (
	contentLengthMatch = `Content-Length:\s*(\d+)`
)

func connectToLSP(host string, port uint) (*net.TCPConn, error) {
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

type receiver struct {
	name     string
	incoming *net.TCPConn
	outgoing *net.TCPConn
	logger   zerolog.Logger
}

func newReceiver(name string, connection *net.TCPConn) *receiver {
	return &receiver{
		name:     name,
		incoming: connection,
		logger:   log.Logger().With().Str(whom, name).Logger(),
	}
}

func (r *receiver) setOutgoing(connection *net.TCPConn) {
	r.outgoing = connection
}

func (r *receiver) receive() {
	content := make([]byte, 65536)
	reader := bufio.NewReader(r.incoming)

	for {
		var contentLen = 0
		for {
			lineBytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				r.logger.Error().Err(err).Msg("Read first line")
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				continue
			} else if isPrefix {
				r.logger.Error().Err(err).Msg("Only beginning of header line read")
				continue
			}
			if len(lineBytes) == 0 {
				break
			}
			re := regexp.MustCompile(contentLengthMatch)
			matches := re.FindStringSubmatch(string(lineBytes))
			if len(matches) < 2 {
				continue
			}
			contentLen, err = strconv.Atoi(matches[1])
			if err != nil {
				r.logger.Error().Err(err).Msgf("Content length '%s' not integer", matches[0])
				continue
			}
		}
		if contentLen == 0 {
			r.logger.Error().Msg("header had no content length")
			continue
		}

		if length, err := reader.Read(content[:contentLen]); err != nil {
			r.logger.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			r.logger.Error().Msgf("read %d bytes instead of %d", length, contentLen)
		} else {
			r.logger.Debug().RawJSON("msg", content[:contentLen]).Msg("Received")
			if r.outgoing != nil {
				if err := sendContent(r.name, content[:contentLen], r.outgoing); err != nil {
					r.logger.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}
