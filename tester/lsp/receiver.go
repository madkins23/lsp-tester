package lsp

import (
	"github.com/madkins23/lsp-tester/tester/data"
	"github.com/madkins23/lsp-tester/tester/message"
)

var (
	receivers = make(map[string]Receiver)
)

func GetReceiver(name string) Receiver {
	return receivers[name]
}

func Receivers() map[string]Receiver {
	return receivers
}

type Receiver interface {
	Handler
	Receive(ready *chan bool)
	SendContent(from, to string, content []byte, msgLogger *message.Logger) error
	SendMessage(to string, message data.AnyMap, msgLogger *message.Logger) error
	SetOther(other Receiver)
	Start() error
}
