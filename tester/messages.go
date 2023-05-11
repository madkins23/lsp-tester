package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

type genericData map[string]any

func (gd genericData) hasField(name string) bool {
	_, found := gd[name]
	return found
}

type message struct {
	JSONRPC string `json:"jsonrpc"`
}

// request represents the
// RequestMessage, NotificationMessage, Registration, and Unregistration messages.
type request struct {
	message
	ID              int    `json:"id"`
	Method          string `json:"method"`
	Params          any    `json:"params"`
	RegisterOptions any    `json:"registerOptions"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type response struct {
	message
	ID     int            `json:"id"`
	Result any            `json:"result"`
	Error  *responseError `json:"error"`
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

	if params, ok := rqst.Params.(genericData); ok {
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
	paramsByID = make(map[int]any)
)

func logMessage(from, to, msg string, content []byte) {
	const (
		leftArrow  = "<--"
		rightArrow = "-->"
	)
	var direction string
	if strings.HasPrefix(from, "client") {
		direction = from + rightArrow + to
	} else if from == "server" {
		direction = to + leftArrow + from
	} else if strings.HasPrefix(to, "client") {
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
		data := make(genericData)
		if err := json.Unmarshal(content, &data); err != nil {
			log.Warn().Err(err).Msg("Unmarshal content")
			// Fall through to end where raw JSON is added.
		} else {
			event = log.Info().Str("!", direction).Int("#size", len(content))
			if err := simpleMessageFormat(data, content, event, msg); err != nil {
				log.Warn().Err(err).Msg("simpleMessageFormat()")
			}
			return
		}
	}

	event.RawJSON("msg", content).Msg(msg)
}

const (
	maxStringDisplayLen = 32
	maxHashDisplayLen   = 64
)

var errUnknownMessageType = errors.New("Unknown Message Type")

func simpleMessageFormat(data genericData, content []byte, event *zerolog.Event, msg string) error {
	msgType := "unknown"
	if data.hasField("method") {
		var rqst request
		if err := json.Unmarshal(content, &rqst); err != nil {
			return fmt.Errorf("unmarshal request: %w", err)
		}
		msgType = "notification"
		if rqst.ID > 0 {
			msgType = "request"
			event.Int("%ID", rqst.ID)
		}
		if rqst.Method != "" {
			event.Str("%method", rqst.Method)
			methodByID[rqst.ID] = rqst.Method
		}
		if rqst.Params != nil {
			addToEventWithLog("params", rqst.Params, event)
			paramsByID[rqst.ID] = rqst.Params
		}
		if rqst.RegisterOptions != nil {
			msgType = "registration"
			addToEventWithLog("options", rqst.RegisterOptions, event)
		}
	} else if data.hasField("result") {
		var resp response
		msgType = "response"
		if err := json.Unmarshal(content, &resp); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
		if resp.ID > 0 {
			event.Int("%ID", resp.ID)
			if method, found := methodByID[resp.ID]; found {
				event.Str("%rqst-method", method)
			}
			if params, found := paramsByID[resp.ID]; found {
				addToEventWithLog("%rqst-params", params, event)
			}
		}
		if resp.Error != nil {
			if resp.Error.Code > 0 {
				addToEventWithLog("error-code", resp.Error.Code, event)
			}
			if resp.Error.Message != "" {
				addToEventWithLog("error-msg", resp.Error.Message, event)
			}
			if resp.Error.Data != nil {
				addToEventWithLog("error-data", resp.Error.Data, event)
			}
		}
		addToEventWithLog("result", resp.Result, event)
	}
	if msgType == "unknown" {
		event.Err(errUnknownMessageType)
		if str, err := marshalAny(data); err != nil {
			return fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str("msg", str)
		}
	} else {
		event.Str("$Type", msgType)
	}
	event.Msg(msg)
	return nil
}

func addToEventWithLog(key string, item any, event *zerolog.Event) {
	if found, err := addToEvent(key, item, event); err != nil {
		log.Warn().Err(err).Msgf("Adding %s to event", key)
	} else if !found {
		log.Debug().Msgf("Empty %s", key)
	}
}

func addToEvent(key string, item any, event *zerolog.Event) (bool, error) {
	added := true
	if text, ok := item.(string); ok {
		if len(text) > maxStringDisplayLen {
			text = text[:maxStringDisplayLen]
		}
		event.Str(key, text)
	} else if number, ok := item.(float64); ok {
		event.Float64(key, number)
	} else if boolean, ok := item.(bool); ok {
		event.Bool(key, boolean)
	} else if hash, ok := item.(map[string]interface{}); ok && len(hash) > 0 {
		for _, attempt := range []string{key, "text", "path", "value", "data"} {
			if something, found := hash[attempt]; found {
				if done, err := addToEvent(key, something, event); err != nil {
					return false, fmt.Errorf("addToEvent: %w", err)
				} else if done {
					return true, nil
				}
			}
		}
		added = false
	}
	if !added {
		if str, err := marshalAny(item); err != nil {
			return false, fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str(key, str)
			added = true
		}
	}
	return true, nil
}

func marshalAny(item any) (string, error) {
	if jsonBytes, err := json.Marshal(item); err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	} else {
		if len(jsonBytes) > maxHashDisplayLen {
			jsonBytes = append(jsonBytes[:maxHashDisplayLen], []byte("...")...)
		}
		return string(jsonBytes), nil
	}
}
