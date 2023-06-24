package lsp

import (
	"bufio"
	"io"
)

type Handler interface {
	Reader() *bufio.Reader
	Writer() io.Writer
	Kill() error
}
