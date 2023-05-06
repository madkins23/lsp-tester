package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"

	"github.com/madkins23/go-utils/log"
)

type message struct {
	JSONRPC string `json:"jsonrpc"`
}

type request struct {
	message
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

type responseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type response struct {
	message
	ID     int           `json:"id"`
	Result interface{}   `json:"result"`
	Error  responseError `json:"error"`
}

func formatMessageJSON(m map[string]interface{}, buffer *bytes.Buffer) error {
	if msg, found := m["msg"]; found {
		if pretty, err := json.MarshalIndent(msg, "", "  "); err != nil {
			return fmt.Errorf("marshal msg JSON: %w", err)
		} else {
			buffer.WriteString("\n")
			buffer.Write(pretty)
		}
	}

	return nil
}

const (
	idRange   = 1000
	msgHdrFmt = "Content-Length: %d\r\n\r\n%s\r\n"
)

func sendMessage(name string, connection *net.TCPConn, rqstPath string) error {
	sendLog := log.Logger().With().Str("who", name).Logger()
	var err error
	var content []byte
	request := &request{}
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

	sendLog.Debug().RawJSON("msg", content).Msg("Send")
	message := fmt.Sprintf(msgHdrFmt, len(content), string(content))
	if _, err = connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}
