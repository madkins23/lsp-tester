package command

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/lsp"
	"github.com/madkins23/lsp-tester/tester/message"
)

var _ lsp.Receiver = (*ProcessReceiver)(nil)

type ProcessReceiver struct {
	*lsp.ReceiverBase
	cmd *exec.Cmd
}

func NewProcess(
	to string, flags *flags.Set, msgLgr *message.Logger, waiter *sync.WaitGroup) (lsp.Receiver, error) {

	ctx, cancel := context.WithCancel(context.Background())
	path, args := flags.Command()
	log.Debug().Str("path", path).Strs("args", args).Msg("execute command")
	cmd := exec.CommandContext(ctx, path, args...)
	procStdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	procStdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	return &ProcessReceiver{
		ReceiverBase: lsp.NewReceiver(to, flags, NewHandler(procStdin, procStdout, cancel), msgLgr, waiter),
		cmd:          cmd,
	}, nil
}

func (pr *ProcessReceiver) Start() error {
	if err := pr.cmd.Start(); err != nil {
		return fmt.Errorf("run command: %w", err)
	} else {
		return pr.ReceiverBase.Start()
	}
}

///////////////////////////////////////////////////////////////////////////////

var _ lsp.Handler = (*Handler)(nil)

type Handler struct {
	writer io.Writer
	reader *bufio.Reader
	cancel context.CancelFunc
}

func NewHandler(input io.Writer, output io.Reader, cancel context.CancelFunc) *Handler {
	return &Handler{
		writer: input,
		reader: bufio.NewReader(output),
		cancel: cancel,
	}
}

func (h *Handler) Reader() *bufio.Reader {
	return h.reader
}

func (h *Handler) Writer() io.Writer {
	return h.writer
}

func (h *Handler) Kill() error {
	h.cancel()
	return nil
}
