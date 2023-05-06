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

type receiver struct {
	partner string
	conn    net.Conn
	other   *receiver
	rcvLog  zerolog.Logger
}

func newReceiver(name, partner string, connection net.Conn) *receiver {
	return &receiver{
		partner: partner,
		conn:    connection,
		rcvLog:  log.Logger().With().Str(whom, name).Logger(),
	}
}

func (r *receiver) receive() {
	content := make([]byte, 1048576) // 1 Mb
	reader := bufio.NewReader(r.conn)

	for {
		var contentLen = 0
		for {
			lineBytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				r.rcvLog.Error().Err(err).Msg("Read first line")
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				continue
			} else if isPrefix {
				r.rcvLog.Error().Err(err).Msg("Only beginning of header line read")
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
				r.rcvLog.Error().Err(err).Msgf("Content length '%s' not integer", matches[0])
				continue
			}
		}
		if contentLen == 0 {
			r.rcvLog.Error().Msg("header had no content length")
			continue
		}

		if length, err := io.ReadFull(reader, content[:contentLen]); err != nil {
			r.rcvLog.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			r.rcvLog.Error().Msgf("Read %d bytes instead of %d", length, contentLen)
		} else {
			r.rcvLog.Debug().
				Str(whoFrom, r.partner).Str(whoTo, "tester").Int(sizeOf, contentLen).
				RawJSON("msg", content[:contentLen]).Msg("Received")
			if r.other != nil {
				if err := r.other.sendContent(content[:contentLen]); err != nil {
					r.rcvLog.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

func (r *receiver) sendContent(content []byte) error {
	if err := sendContent(r.partner, content, r.conn, &r.rcvLog); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}
