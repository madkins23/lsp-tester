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
	idRandomRange   = 1000
	msgHeaderFormat = "Content-Length: %d\r\n\r\n%s\r\n"
	jsonRpcVersion  = "2.0"
)

func loadRequest(requestPath string) (*request, error) {
	var err error
	var content []byte
	rqst := &request{}
	if content, err = os.ReadFile(requestPath); err != nil {
		return nil, fmt.Errorf("read request %s: %w", requestPath, err)
	}
	if err = json.Unmarshal(content, rqst); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}
	return rqst, nil
}

func sendRequest(from string, rqst *request, connection *net.TCPConn) error {
	rqst.JSONRPC = jsonRpcVersion
	rqst.ID = rand.Intn(idRandomRange)

	if params, ok := rqst.Params.(map[string]interface{}); ok {
		if path, found := params["path"]; found {
			if relPath, ok := path.(string); ok {
				if absPath, err := filepath.Abs(relPath); err == nil {
					params["path"] = absPath
				}
			}
		}
	}

	if content, err := json.Marshal(rqst); err != nil {
		return fmt.Errorf("marshal request: %w", err)
	} else if err := sendContent(from, content, connection); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

func sendContent(from string, content []byte, connection *net.TCPConn) error {
	log.Debug().Str(whom, from).RawJSON("msg", content).Msg("Send")
	message := fmt.Sprintf(msgHeaderFormat, len(content), string(content))
	if _, err := connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}
