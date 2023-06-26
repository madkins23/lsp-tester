package sub

import (
	"bufio"
	"io"
	"os"
	"sync"

	"github.com/madkins23/go-utils/app"

	"github.com/madkins23/lsp-tester/tester/protocol/lsp"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/message"
)

func NewCaller(to string, flags *flags.Set,
	msgLgr *message.Logger, waiter *sync.WaitGroup, terminator *app.Terminator) lsp.Receiver {
	//
	return lsp.NewReceiver(
		to, flags, NewCallerHandler(os.Stdout, os.Stdin), msgLgr, waiter, terminator)
}

///////////////////////////////////////////////////////////////////////////////

var _ lsp.Handler = (*CallerHandler)(nil)

type CallerHandler struct {
	writer io.Writer
	reader *bufio.Reader
}

func NewCallerHandler(input io.Writer, output io.Reader) *CallerHandler {
	return &CallerHandler{
		writer: input,
		reader: bufio.NewReader(output),
	}
}

func (h *CallerHandler) Reader() *bufio.Reader {
	return h.reader
}

func (h *CallerHandler) Writer() io.Writer {
	return h.writer
}

func (h *CallerHandler) Kill() error {
	return nil
}
