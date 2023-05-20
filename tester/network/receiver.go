package network

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/madkins23/go-utils/log"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/message"
)

const (
	contentLengthMatch = `Content-Length:\s*(\d+)`
)

var (
	sequence atomic.Uint32
)

var (
	receivers = make(map[string]*Receiver)
)

func GetReceiver(name string) *Receiver {
	return receivers[name]
}

func Receivers() map[string]*Receiver {
	return receivers
}

type Receiver struct {
	flags  *flags.Set
	to     string
	conn   net.Conn
	other  *Receiver
	msgLgr *message.Logger
	waiter *sync.WaitGroup
}

func NewReceiver(to string, flags *flags.Set, connection net.Conn, msgLgr *message.Logger, waiter *sync.WaitGroup) *Receiver {
	if to == "client" {
		to += "-" + strconv.Itoa(int(sequence.Add(1)))
	}
	return &Receiver{
		flags:  flags,
		to:     to,
		conn:   connection,
		msgLgr: msgLgr,
		waiter: waiter,
	}
}

func (r *Receiver) Connection() net.Conn {
	return r.conn
}

func (r *Receiver) Start() error {
	ready := make(chan bool)
	go r.Receive(&ready)
	for i := 0; i < 5; i++ {
		select {
		case <-ready:
			log.Debug().Str("to", r.to).Msg("Connected")
			return nil
		case <-time.After(time.Second):
			log.Debug().Str("to", r.to).Msg("Connecting...")
		}
	}
	return fmt.Errorf("connection to %s not made", r.to)
}

func (r *Receiver) SetOther(other *Receiver) {
	r.other = other
}

func (r *Receiver) Receive(ready *chan bool) {
	log.Info().Str("to", r.to).Msg("Receiver starting")
	defer log.Info().Str("to", r.to).Msg("Receiver finished")

	receivers[r.to] = r
	defer delete(receivers, r.to)

	r.waiter.Add(1)
	defer r.waiter.Done()

	content := make([]byte, 1048576) // 1 Mb
	reader := bufio.NewReader(r.conn)

	// Notify caller Receiver is about to do its thing.
	*ready <- true

	for {
		contentLen := r.receiveMsg(reader)
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
			if r.other == nil {
				r.msgLgr.Message(r.to, "tester", "Rcvd", content)
			} else {
				// TODO: What if there are multiple clients?
				// How do we know which one server should send to?
				from := r.to
				if r.flags.LogMessageTwice() {
					from = "tester"
					r.msgLgr.Message(r.to, "tester", "Rcvd", content)
				}
				if err := r.other.sendContent(from, content); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

func (r *Receiver) receiveMsg(reader *bufio.Reader) int {
	var contentLen = 0
	for {
		lineBytes, isPrefix, err := reader.ReadLine()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
				return 0
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
	return contentLen
}

func (r *Receiver) sendContent(from string, content []byte) error {
	if err := message.SendContent(from, r.to, content, r.conn, r.msgLgr); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}

func (r *Receiver) Kill() error {
	return r.conn.Close()
}
