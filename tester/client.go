package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/madkins23/go-utils/log"
)

const (
	idRange   = 1000
	msgHdrFmt = "Content-Length: %d\r\n\r\n%s\r\n"
)

func client(host string, port uint, rqstPath string) error {
	log.Info().Msg("LSP client starting")
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

	go receiver("client", connection)

	var err error
	var content []byte
	request := &Request{}
	if content, err = os.ReadFile(rqstPath); err != nil {
		return fmt.Errorf("read request %s: %w", rqstPath, err)
	}
	if err = json.Unmarshal(content, request); err != nil {
		return fmt.Errorf("unmarshal request: %w", err)
	}

	request.JSONRPC = JSON_RPC
	request.ID = rand.Intn(idRange)

	if params, ok := request.Params.(map[string]interface{}); ok {
		if path, found := params["path"]; found {
			if relPath, ok := path.(string); ok {
				if absPath, err := filepath.Abs(relPath); err == nil {
					params["path"] = absPath
				}
			}
		}
	}

	if content, err = json.Marshal(request); err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	log.Debug().RawJSON("msg", content).Msg("Send")
	message := fmt.Sprintf(msgHdrFmt, len(content), string(content))
	if _, err = connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	time.Sleep(time.Hour)

	return nil
}

func extraJson(m map[string]interface{}, buffer *bytes.Buffer) error {
	if msg, found := m["msg"]; found {
		//if msgBytes, ok := msg.([]byte); ok {
		//	data := make(map[string]interface{})
		//if err := json.Unmarshal(msgBytes, &data); err != nil {
		//	return fmt.Errorf("unmarshal msg JSON: %w", err)
		//} else
		if pretty, err := json.MarshalIndent(msg, "", "  "); err != nil {
			return fmt.Errorf("marshal msg JSON: %w", err)
		} else {
			buffer.WriteString("\n")
			buffer.Write(pretty)
		}
		//}
	}

	return nil
}

func receiver(name string, connection *net.TCPConn) {
	rcvrLog := log.Logger().With().Str("who", name).Logger()
	reader := bufio.NewReader(connection)

	for {
		var contentLen = 0
		for {
			lineBytes, isPrefix, err := reader.ReadLine()
			if err != nil {
				rcvrLog.Error().Err(err).Msg("Read first line")
				if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) {
					return
				}
				continue
			} else if isPrefix {
				rcvrLog.Error().Err(err).Msg("Only beginning of header line read")
				continue
			}
			if len(lineBytes) == 0 {
				break
			}
			re := regexp.MustCompile(`Content-Length:\s*(\d+)`)
			matches := re.FindStringSubmatch(string(lineBytes))
			if len(matches) < 2 {
				continue
			}
			contentLen, err = strconv.Atoi(matches[1])
			if err != nil {
				rcvrLog.Error().Err(err).Msgf("Content length '%s' not integer", matches[0])
				continue
			}
		}
		if contentLen == 0 {
			rcvrLog.Error().Msg("header had no content length")
			continue
		}

		reply := make([]byte, 65536)

		if length, err := reader.Read(reply[:contentLen]); err != nil {
			rcvrLog.Error().Err(err).Msg("Read response")
		} else if length != contentLen {
			rcvrLog.Error().Msgf("read %d bytes instead of %d", length, contentLen)
		} else {
			rcvrLog.Debug().RawJSON("msg", reply[:contentLen]).Msg("Received")
		}
	}
}
