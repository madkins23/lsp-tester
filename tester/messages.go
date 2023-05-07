package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"

	"github.com/rs/zerolog"
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

//type response struct {
//	message
//	ID     int           `json:"id"`
//	Result interface{}   `json:"result"`
//	Error  responseError `json:"error"`
//}

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
	msgHeaderFormat = "Content-Length: %d\r\n\r\n%s"
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

func sendRequest(who, to string, rqst *request, connection net.Conn, logger *zerolog.Logger) error {
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
	} else if err := sendContent(to, content, connection, logger); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

func sendContent(to string, content []byte, connection net.Conn, logger *zerolog.Logger) error {
	logMessage("tester", to, "Send", content, logger)
	message := fmt.Sprintf(msgHeaderFormat, len(content), string(content))
	if _, err := connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}

var (
	methodByID = make(map[int]string)
	paramByID  = make(map[int]string)
)

func logMessage(from, to, msg string, content []byte, logger *zerolog.Logger) {
	event := logger.Debug().Str(whoFrom, from).Str(whoTo, to).Int(sizeOf, len(content))

	if simpleFmt {
		data := make(map[string]interface{})
		if err := json.Unmarshal(content, &data); err != nil {
			logger.Warn().Err(err).Msg("Unmarshal content")
			// Fall through to end where raw JSON is added.
		} else {
			var intID int
			var methodFound bool
			var paramFound bool
			if id, found := data["id"]; found {
				if number, ok := id.(float64); ok {
					intID = int(number)
					event.Int("id", intID)
				}
			}
			if method, found := data["method"]; found {
				if name, ok := method.(string); ok {
					methodFound = true
					event.Str("method", name)
					if intID > 0 {
						methodByID[intID] = name
					}
				}
			}
			if !methodFound && intID > 0 {
				if method, found := methodByID[intID]; found {
					event.Str("from-method", method)
				}
			}
			if params, found := data["params"]; found {
				if paramData, ok := params.(map[string]interface{}); ok {
					if paramText, found := paramData["text"]; found {
						if text, ok := paramText.(string); ok {
							if len(text) > 32 {
								text = text[:32]
							}
							paramFound = true
							event.Str("param", text)
							if intID > 0 {
								paramByID[intID] = text
							}
						}
					}
				}
			}
			if !paramFound && intID > 0 {
				if param, found := paramByID[intID]; found {
					event.Str("method-param", param)
				}
			}
			if result, found := data["result"]; found {
				if resultData, ok := result.(map[string]interface{}); ok {
					if resultText, found := resultData["text"]; found {
						if text, ok := resultText.(string); ok {
							if len(text) > 32 {
								text = text[:32]
							}
							event.Str("result", text)
						}
					}
				} else if resultBytes, err := json.Marshal(result); err != nil {
					logger.Warn().Err(err).Msg("Marshal result")
				} else {
					if len(resultBytes) > 64 {
						resultBytes = resultBytes[:64]
					}
					event.Str("result", string(resultBytes)+"...")
				}
			}
			event.Msg(msg)
			return
		}
	}

	event.RawJSON("msg", content).Msg(msg)
}
