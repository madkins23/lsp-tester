package lsp

import (
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	// TODO: Does this need to be go-utils/log as opposed to zerolog?
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

///////////////////////////////////////////////////////////////////////////////

type Receiver struct {
	Handler
	flags  *flags.Set
	to     string
	other  *Receiver
	msgLgr *message.Logger
	waiter *sync.WaitGroup
}

func NewReceiver(to string, flags *flags.Set, handler Handler, msgLgr *message.Logger, waiter *sync.WaitGroup) *Receiver {
	if to == "client" {
		to += "-" + strconv.Itoa(int(sequence.Add(1)))
	}
	return &Receiver{
		Handler: handler,
		flags:   flags,
		to:      to,
		msgLgr:  msgLgr,
		waiter:  waiter,
	}
}

func (lsp *Receiver) Start() error {
	ready := make(chan bool)
	go lsp.Receive(&ready)
	for i := 0; i < 5; i++ {
		select {
		case <-ready:
			log.Debug().Str("to", lsp.to).Msg("Connected")
			return nil
		case <-time.After(time.Second):
			log.Debug().Str("to", lsp.to).Msg("Connecting...")
		}
	}
	return fmt.Errorf("connection to %s not made", lsp.to)
}

func (lsp *Receiver) SetOther(other *Receiver) {
	lsp.other = other
}

func (lsp *Receiver) Receive(ready *chan bool) {
	log.Info().Str("to", lsp.to).Msg("Receiver starting")
	defer log.Info().Str("to", lsp.to).Msg("Receiver finished")

	receivers[lsp.to] = lsp
	defer delete(receivers, lsp.to)

	lsp.waiter.Add(1)
	defer lsp.waiter.Done()

	content := make([]byte, 1048576) // 1 Mb

	// Notify caller Receiver is about to do its thing.
	*ready <- true

	for {
		contentLen := lsp.receiveMsg()
		if contentLen == 0 {
			log.Error().Msg("header had no content length")
			continue
		}

		if length, err := io.ReadFull(lsp.Reader(), content[:contentLen]); err != nil {
			log.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			log.Error().Msgf("Read %d bytes instead of %d", length, contentLen)
		} else {
			content = content[:contentLen]
			if lsp.other == nil {
				lsp.msgLgr.Message(lsp.to, "tester", "Rcvd", content)
			} else {
				// TODO: What if there are multiple clients?
				// How do we know which one server should send to?
				from := lsp.to
				if lsp.flags.LogMessageTwice() {
					from = "tester"
					lsp.msgLgr.Message(lsp.to, "tester", "Rcvd", content)
				}
				if err := lsp.other.sendContent(from, content); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

//-----------------------------------------------------------------------------

func (lsp *Receiver) receiveMsg() int {
	var contentLen = 0
	for {
		lineBytes, isPrefix, err := lsp.Reader().ReadLine()
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

func (lsp *Receiver) sendContent(from string, content []byte) error {
	if err := message.SendContent(from, lsp.to, content, lsp.Writer(), lsp.msgLgr); err != nil {
		return fmt.Errorf("send content: %w", err)
	} else {
		return nil
	}
}
