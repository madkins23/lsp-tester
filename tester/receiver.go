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

func receiver(name string, connection *net.TCPConn) {
	rcvrLog := log.Logger().With().Str("who", name).Logger()
	reader := bufio.NewReader(connection)

	for {
		var contentLen = 0
		for {
			lineBytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				rcvrLog.Error().Err(err).Msg("Read first line")
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				continue
			} else if isPrefix {
				rcvrLog.Error().Err(err).Msg("Only beginning of header line read")
				continue
			}
			if len(lineBytes) == 0 {
				break
			}
			re := regexp.MustCompile(`Content-Length:\s*(\d+)`)
			matches := re.FindStringSubmatch(string(lineBytes))
			if len(matches) < 2 {
				continue
			}
			contentLen, err = strconv.Atoi(matches[1])
			if err != nil {
				rcvrLog.Error().Err(err).Msgf("Content length '%s' not integer", matches[0])
				continue
			}
		}
		if contentLen == 0 {
			rcvrLog.Error().Msg("header had no content length")
			continue
		}

		reply := make([]byte, 65536)

		if length, err := reader.Read(reply[:contentLen]); err != nil {
			rcvrLog.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			rcvrLog.Error().Msgf("read %d bytes instead of %d", length, contentLen)
		} else {
			rcvrLog.Debug().RawJSON("msg", reply[:contentLen]).Msg("Received")
		}
	}
}
