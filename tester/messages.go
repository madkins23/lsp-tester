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

//type responseError struct {
//	Code    int         `json:"code"`
//	Message string      `json:"message"`
//	Data    interface{} `json:"data"`
//}

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

func loadMessageFiles() error {
	if messageDir == "" {
		// Nothing to do here
	} else if entries, err := os.ReadDir(messageDir); err != nil {
		messages = make([]string, 0)
		return fmt.Errorf("read message directory %s: %w", messageDir, err)
	} else {
		messages = make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				messages = append(messages, entry.Name())
			}
		}
	}
	return nil
}

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

func sendRequest(to string, rqst *request, connection net.Conn) error {
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
	} else if err := sendContent(to, content, connection); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

func sendContent(to string, content []byte, connection net.Conn) error {
	logMessage("tester", to, "Send", content)
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

func logMessage(from, to, msg string, content []byte) {
	const (
		leftArrow  = "<--"
		rightArrow = "-->"
	)
	var direction string
	if from == "client" {
		direction = from + rightArrow + to
	} else if from == "server" {
		direction = to + leftArrow + from
	} else if to == "client" {
		direction = to + leftArrow + from
	} else if to == "server" {
		direction = from + rightArrow + to
	}
	if direction == "" {
		log.Warn().Str("from", from).Str("to", to).Msg("Uncertain direction")
		direction = from + leftArrow + to
	}

	event := log.Info().Str("!", direction).Int("#size", len(content))

	if simpleFormat {
		data := make(map[string]interface{})
		if err := json.Unmarshal(content, &data); err != nil {
			log.Warn().Err(err).Msg("Unmarshal content")
			// Fall through to end where raw JSON is added.
		} else {
			var ID int
			if id, found := data["id"]; found {
				if number, ok := id.(float64); ok {
					ID = int(number)
					event.Int("$ID", ID)
				}
			}

			// TODO: Is there a good way to find a field exists and
			// then unmarshal the data into a specific struct for better output?
			// https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification
			addSomething("method", data, event, methodByID, ID, "from-method")
			addSomething("params", data, event, paramByID, ID, "method-params")
			addSomething("result", data, event, nil, ID, "")
			addSomething("error", data, event, nil, ID, "")
			event.Msg(msg)
			return
		}
	}

	event.RawJSON("msg", content).Msg(msg)
}

const (
	maxStringDisplayLen = 32
	maxHashDisplayLen   = 64
)

func addSomething(
	key string, item interface{}, event *zerolog.Event,
	remember map[int]string, id int, notFoundKey string,
) bool {
	var foundSomething bool
	if text, ok := item.(string); ok {
		if len(text) > maxStringDisplayLen {
			text = text[:maxStringDisplayLen]
		}
		event.Str(key, text)
		if remember != nil {
			remember[id] = text
		}
		foundSomething = true
	} else if number, ok := item.(float64); ok {
		event.Float64(key, number)
		foundSomething = true
	} else if boolean, ok := item.(bool); ok {
		event.Bool(key, boolean)
		foundSomething = true
	} else if hash, ok := item.(map[string]interface{}); ok && len(hash) > 0 {
		for _, attempt := range []string{key, "text", "path", "value", "data"} {
			if something, found := hash[attempt]; found {
				foundSomething = addSomething(key, something, event, remember, id, notFoundKey)
				if foundSomething {
					break
				} else if someHash, ok := something.(map[string]interface{}); ok && len(someHash) > 0 {
					if resultBytes, err := json.Marshal(someHash); err != nil {
						log.Warn().Err(err).Msg("Marshal result")
					} else {
						if len(resultBytes) > maxHashDisplayLen {
							resultBytes = append(resultBytes[:maxHashDisplayLen], []byte("...")...)
						}
						event.Bytes(key, resultBytes)
						foundSomething = true
					}
				}
			}
		}
	}
	if id > 0 && !foundSomething {
		if something, found := remember[id]; found {
			event.Str(notFoundKey, something)
		}
	}
	return foundSomething
}
