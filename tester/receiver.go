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
	"time"

	"github.com/madkins23/go-utils/log"
)

const (
	contentLengthMatch = `Content-Length:\s*(\d+)`
)

var (
	sequence atomic.Uint32
)

type receiver struct {
	to    string
	conn  net.Conn
	other *receiver
}

func startReceiver(to string, connection net.Conn) (*receiver, error) {
	if to == "client" {
		to += "-" + strconv.Itoa(int(sequence.Add(1)))
	}
	rcvr := &receiver{
		to:   to,
		conn: connection,
	}
	receivers[to] = rcvr
	ready := make(chan bool)
	go rcvr.receive(&ready)
	for i := 0; i < 5; i++ {
		select {
		case <-ready:
			log.Debug().Str("to", to).Msg("Connected")
			return rcvr, nil
		case <-time.After(time.Second):
			log.Debug().Str("to", to).Msg("Connecting...")
		}
	}
	return nil, fmt.Errorf("connection to %s not made", to)
}

func (r *receiver) kill() error {
	return r.conn.Close()
}

func (r *receiver) receive(ready *chan bool) {
	log.Info().Str("to", r.to).Msg("Receiver starting")
	waiter.Add(1)
	defer func() {
		log.Info().Str("to", r.to).Msg("Receiver finished")
		delete(receivers, r.to)
		waiter.Done()
	}()

	content := make([]byte, 1048576) // 1 Mb
	reader := bufio.NewReader(r.conn)

	// Notify caller receiver is about to do its thing.
	*ready <- true

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
			content = content[:contentLen]
			logMessage(r.to, "tester", "Rcvd", content)
			// TODO: What if there are multiple clients?
			// How do we know which one server should send to?
			if r.other != nil {
				if err := r.other.sendContent(content); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

func (r *receiver) sendContent(content []byte) error {
	if err := sendContent(r.to, content, r.conn); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}
