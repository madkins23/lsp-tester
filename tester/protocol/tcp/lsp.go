package tcp

import (
	"fmt"
	"net"
	"strconv"

	"github.com/madkins23/lsp-tester/tester/flags"
)

func ConnectToLSP(flags *flags.Set) (net.Conn, error) {
	tcpAddress := flags.HostAddress() + ":" + strconv.Itoa(flags.ServerPort())
	var connection *net.TCPConn

	if tcpAddr, err := net.ResolveTCPAddr("tcp", tcpAddress); err != nil {
		return nil, fmt.Errorf("resolve TCP address: %w", err)
	} else if connection, err = net.DialTCP("tcp", nil, tcpAddr); err != nil {
		return nil, fmt.Errorf("dial TCP address: %w", err)
	} else {
		return connection, nil
	}
}
