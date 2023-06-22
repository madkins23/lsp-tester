package lsp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/madkins23/lsp-tester/tester/data"
	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/message"
)

var _ Receiver = (*ReceiverBase)(nil)

type ReceiverBase struct {
	Handler
	flags  *flags.Set
	to     string
	other  Receiver
	msgLgr *message.Logger
	waiter *sync.WaitGroup
}

var sequence atomic.Uint32

func NewReceiver(to string, flags *flags.Set, handler Handler, msgLgr *message.Logger, waiter *sync.WaitGroup) *ReceiverBase {
	if to == "client" {
		to += "-" + strconv.Itoa(int(sequence.Add(1)))
	}
	return &ReceiverBase{
		Handler: handler,
		flags:   flags,
		to:      to,
		msgLgr:  msgLgr,
		waiter:  waiter,
	}
}

func (lsp *ReceiverBase) Start() error {
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

func (lsp *ReceiverBase) SetOther(other Receiver) {
	lsp.other = other
}

func (lsp *ReceiverBase) Receive(ready *chan bool) {
	log.Info().Str("to", lsp.to).Msg("ReceiverBase starting")
	defer log.Info().Str("to", lsp.to).Msg("ReceiverBase finished")

	receivers[lsp.to] = lsp
	defer delete(receivers, lsp.to)

	lsp.waiter.Add(1)
	defer lsp.waiter.Done()

	content := make([]byte, 1048576) // 1 Mb

	// Notify caller ReceiverBase is about to do its thing.
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
				if err := lsp.other.SendContent(from, lsp.to, content, lsp.msgLgr); err != nil {
					log.Error().Err(err).Msg("Sending outgoing message")
				}
			}
		}
	}
}

const (
	idRandomRange  = 1000
	jsonRpcVersion = "2.0"
)

// SendMessage marshals a data.AnyMap object and sends it to the specified connection.
// The data object is edited to contain a JSON RPC version, a request ID,
// and contained relative path fields are replaced with absolute paths.
func (lsp *ReceiverBase) SendMessage(to string, message data.AnyMap, msgLgr *message.Logger) error {
	message["jsonrpc"] = jsonRpcVersion
	message["id"] = strconv.Itoa(idRandomRange + rand.Intn(idRandomRange))
	//message["id"] = idRandomRange + rand.Intn(idRandomRange)

	if params, ok := message["params"].(data.AnyMap); ok {
		if path, found := params["path"]; found {
			if relPath, ok := path.(string); ok {
				if absPath, err := filepath.Abs(relPath); err == nil {
					params["path"] = absPath
				}
			}
		}
	}

	if content, err := json.Marshal(message); err != nil {
		return fmt.Errorf("marshal request: %w", err)
	} else if err := lsp.SendContent("tester", to, content, msgLgr); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

const msgHeaderFormat = "Content-Length: %d\r\n\r\n%s"

// SendContent sends byte array content via the specified lsp.Handler.
func (lsp *ReceiverBase) SendContent(from, to string, content []byte, msgLgr *message.Logger) error {
	msgLgr.Message(from, to, "Send", content)
	msg := fmt.Sprintf(msgHeaderFormat, len(content), string(content))
	if _, err := lsp.Writer().Write([]byte(msg)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}
	return nil
}

//-----------------------------------------------------------------------------

const contentLengthMatch = `Content-Length:\s*(\d+)`

func (lsp *ReceiverBase) receiveMsg() int {
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
