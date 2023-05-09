package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync/atomic"

	"github.com/madkins23/go-utils/log"
)

const (
	contentLengthMatch = `Content-Length:\s*(\d+)`
)

var (
	sequence atomic.Uint32
)

type receiver struct {
	connectedTo string
	conn        net.Conn
	other       *receiver
}

func newReceiver(connectedTo string, connection net.Conn) *receiver {
	if connectedTo == "client" {
		connectedTo += "-" + strconv.Itoa(int(sequence.Add(1)))
	}
	rcvr := &receiver{
		connectedTo: connectedTo,
		conn:        connection,
	}
	receivers[connectedTo] = rcvr
	return rcvr
}

func (r *receiver) kill() error {
	return r.conn.Close()
}

func (r *receiver) receive() {
	log.Info().Str("connection", r.connectedTo).Msg("Receiver starting")
	waiter.Add(1)
	defer func() {
		log.Info().Str("connection", r.connectedTo).Msg("Receiver finished")
		delete(receivers, r.connectedTo)
		waiter.Done()
	}()

	content := make([]byte, 1048576) // 1 Mb
	reader := bufio.NewReader(r.conn)

	for {
		var contentLen = 0
		for {
			lineBytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				log.Error().Err(err).Msg("Read first line")
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
			logMessage(r.connectedTo, "tester", "Rcvd", content[:contentLen])
			if r.other != nil {
				if err := r.other.sendContent(content[:contentLen]); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

func (r *receiver) sendContent(content []byte) error {
	if err := sendContent(r.connectedTo, content, r.conn); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}
