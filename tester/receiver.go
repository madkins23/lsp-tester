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
}

func newReceiver(name, partner string, connection net.Conn) *receiver {
	return &receiver{
		partner: partner,
		conn:    connection,
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
				log.Error().Err(err).Msg("Read first line")
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				continue
			} else if isPrefix {
				log.Error().Err(err).Msg("Only beginning of header line read")
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
				log.Error().Err(err).Msgf("Content length '%s' not integer", matches[0])
				continue
			}
		}
		if contentLen == 0 {
			log.Error().Msg("header had no content length")
			continue
		}

		if length, err := io.ReadFull(reader, content[:contentLen]); err != nil {
			log.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			log.Error().Msgf("Read %d bytes instead of %d", length, contentLen)
		} else {
			logMessage(r.partner, "tester", "Received", content[:contentLen], log.Logger())
			if r.other != nil {
				if err := r.other.sendContent(content[:contentLen]); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

func (r *receiver) sendContent(content []byte) error {
	if err := sendContent(r.partner, content, r.conn, log.Logger()); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}
