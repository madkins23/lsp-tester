package network

import (
	"bufio"
	"io"
	"net"
	"sync"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/lsp"
	"github.com/madkins23/lsp-tester/tester/message"
)

func NewReceiver(to string, flags *flags.Set, connection net.Conn, msgLgr *message.Logger, waiter *sync.WaitGroup) *lsp.Receiver {
	return lsp.NewReceiver(to, flags, NewHandler(connection), msgLgr, waiter)
}

///////////////////////////////////////////////////////////////////////////////

var _ lsp.Handler = (*Handler)(nil)

type Handler struct {
	connection net.Conn
	reader     *bufio.Reader
}

func NewHandler(connection net.Conn) *Handler {
	return &Handler{
		connection: connection,
		reader:     bufio.NewReader(connection),
	}
}

func (h *Handler) Reader() *bufio.Reader {
	return h.reader
}

func (h *Handler) Writer() io.Writer {
	return h.connection
}

func (h *Handler) Kill() error {
	return h.connection.Close()
}
