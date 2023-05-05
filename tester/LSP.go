package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strconv"
	"sync"

	"github.com/madkins23/go-utils/log"
)

var waiter sync.WaitGroup

func main() {
	var (
		hostAddress string
		clientPort  uint
		serverPort  uint
		requestPath string
	)

	flags := flag.NewFlagSet("LSP", flag.ContinueOnError)
	flags.StringVar(&hostAddress, "host", "127.0.0.1", "Host address")
	flags.StringVar(&requestPath, "request", "", "Path to requestPath file (client mode)")
	flags.UintVar(&clientPort, "clientPort", 0, "Client port number")
	flags.UintVar(&serverPort, "serverPort", 0, "Server port number")

	logConfig := log.ConsoleOrFile{}
	logConfig.AddFlagsToSet(flags, "/tmp/LSP.log")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Printf("ResponseError parsing command line flags: %s.", err)
			flags.Usage()
		}
		return
	}

	if err := logConfig.Setup(); err != nil {
		fmt.Printf("Log file creation error: %s.", err)
		return
	}
	defer logConfig.CloseForDefer()

	log.Info().Msg("LSP starting")
	defer log.Info().Msg("LSP finished")

	if clientPort > 0 {
		waiter.Add(1)
		go func() {
			if err := client(hostAddress, clientPort, requestPath); err != nil {
				log.Error().Err(err).Msg("ResponseError in client.")
			}
		}()
	}

	waiter.Wait()
}

const (
	JSON_RPC = "2.0"
	ID_RANGE = 1000
	MESSAGE  = "Content-Length: %d\r\n\r\n%s\r\n"
)

func client(host string, port uint, rqstPath string) error {
	log.Info().Msg("LSP client starting.")
	defer log.Info().Msg("LSP client finished")

	tcpAddress := host + ":" + strconv.Itoa(int(port))
	var connection *net.TCPConn

	if tcpAddr, err := net.ResolveTCPAddr("tcp", tcpAddress); err != nil {
		return fmt.Errorf("resolve TCP address: %w", err)
	} else if connection, err = net.DialTCP("tcp", nil, tcpAddr); err != nil {
		return fmt.Errorf("dial TCP address: %w", err)
	} else {
		defer func() {
			if err := connection.Close(); err != nil {
				log.Error().Err(err).Msg("ResponseError closing connection")
			}
		}()
	}

	if path, err := os.Getwd(); err != nil {
		return fmt.Errorf("get cwd: %w", err)
	} else {
		log.Info().Msgf("cwd: %s", path)
	}

	var err error
	var content []byte
	request := &Request{}
	if content, err = os.ReadFile(rqstPath + ".json"); err != nil {
		return fmt.Errorf("read request %s: %w", rqstPath, err)
	}
	if err = json.Unmarshal(content, request); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	request.JSONRPC = JSON_RPC
	request.ID = rand.Intn(ID_RANGE)
	if content, err = json.Marshal(request); err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	log.Debug().RawJSON("msg", content).Msg("Send")
	message := fmt.Sprintf(MESSAGE, len(content), string(content))
	if _, err = connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	for {
		var contentLen = 0
		reader := bufio.NewReader(connection)
		for {
			bytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				return fmt.Errorf("read first line: %w", err)
			} else if isPrefix {
				return fmt.Errorf("only beginning of header line read")
			}
			if len(bytes) == 0 {
				break
			}
			re := regexp.MustCompile(`Content-Length:\s*(\d+)`)
			matches := re.FindStringSubmatch(string(bytes))
			if len(matches) < 2 {
				continue
			}
			contentLen, err = strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("content length '%s' not integer: %w", matches[0], err)
			}
		}
		if contentLen == 0 {
			return fmt.Errorf("header had not content length")
		}

		reply := make([]byte, 65536)

		if length, err := reader.Read(reply[:contentLen]); err != nil {
			return fmt.Errorf("read response: %w", err)
		} else if length != contentLen {
			return fmt.Errorf("read %d bytes instead of %d", length, contentLen)
		} else {
			log.Debug().RawJSON("msg", reply[:contentLen]).Msg("Received")
		}
	}

	waiter.Done()

	return nil
}

func server(port uint) error {
	log.Info().Msg("LSP server mode.")

	return nil
}
