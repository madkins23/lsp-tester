package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

// formatMessageJSON is a FormatExtra function for zerolog.ConsoleWriter.
// When used it formats the "msg" field as JSON on the lines after the log entry.
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

// loadMessageFiles loads all message files in the message directory.
// The message directory is specified by the messageDir global variable.
// The message file data is stored in the messages global variable as strings.
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

// loadMessage loads the file at the specified path, unmarshals the JSON content,
// and returns a genericData object.
func loadMessage(requestPath string) (genericData, error) {
	var err error
	var content []byte
	var rqst genericData
	if content, err = os.ReadFile(requestPath); err != nil {
		return nil, fmt.Errorf("read request %s: %w", requestPath, err)
	}
	if err = json.Unmarshal(content, &rqst); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}
	return rqst, nil
}

// sendMessage marshals a genericData object and sends it to the specified connection.
// The data object is edited to contain a JSON RPC version, a request ID,
// and contained relative path fields are replaced with absolute paths.
func sendMessage(to string, message genericData, connection net.Conn) error {
	message["jsonrpc"] = jsonRpcVersion
	message["id"] = strconv.Itoa(idRandomRange + rand.Intn(idRandomRange))

	if params, ok := message["params"].(genericData); ok {
		if path, found := params["path"]; found {
			if relPath, ok := path.(string); ok {
				if absPath, err := filepath.Abs(relPath); err == nil {
					params["path"] = absPath
				}
			}
		}
	}

	if content, err := json.Marshal(message); err != nil {
		return fmt.Errorf("marshal request: %w", err)
	} else if err := sendContent(to, content, connection); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

// sendContent sends byte array content to the specified connection.
// A message header is provided before the content.
func sendContent(to string, content []byte, connection net.Conn) error {
	logMessage("tester", to, "Send", content)
	message := fmt.Sprintf(msgHeaderFormat, len(content), string(content))
	if _, err := connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}

func logMessage(from, to, msg string, content []byte) {
	logMessageTo(from, to, msg, content, stdLogger, stdFormat)
	logMessageTo(from, to, msg, content, fileLogger, fileFormat)
}

const (
	leftArrow  = "<--"
	rightArrow = "-->"
)

func logMessageTo(from, to, msg string, content []byte, logger zerolog.Logger, format string) {
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

	event := logger.Info().Str("!", direction).Int("#size", len(content))

	if format == fmtKeyword {
		data := make(genericData)
		if err := json.Unmarshal(content, &data); err != nil {
			log.Warn().Err(err).Msg("Unmarshal content")
			// Fall through to end where raw JSON is added.
		} else {
			event = logger.Info().Str("!", direction).Int("#size", len(content))
			if err := keywordMessageFormat(data, event, msg); err != nil {
				log.Warn().Err(err).Msg("keywordMessageFormat()")
			}
			return
		}
	}

	event.RawJSON("msg", content).Msg(msg)
}

// Keep ID maps below for this long before deleting them.
const idExpiration = 5 * time.Second

var (
	methodByID = make(map[any]string)
	paramsByID = make(map[any]any)
	expireByID = make(map[any]time.Time)
	expireGCgo bool
)

// expirationGC cleans up expireByID, methodByID, and paramsByID
// to avoid having them fill up all available memory over time.
func expirationGC() {
	// Note: Don't worry about graceful shutdown,
	// this goroutine will go away when the application dies.
	// It will not hold up any normal shutdown process.
	for {
		now := time.Now()
		for id, expires := range expireByID {
			if now.After(expires) {
				log.Trace().Any("ID", id).Msg("Delete expiration")
				delete(expireByID, id)
				delete(methodByID, id)
				delete(paramsByID, id)
			}
		}
		time.Sleep(idExpiration)
	}
}

// Make this a command line flag.
const maxDisplayLen = 32

func keywordMessageFormat(data genericData, event *zerolog.Event, msg string) error {
	var msgType string
	if method, found := data.getStringField("method"); found {
		event.Str("%method", method)
		msgType = "notification"
		id, idFound := data.getField("id")
		if idFound {
			msgType = "request"
			event.Any("%ID", id)
			methodByID[id] = method
			expireByID[id] = time.Now().Add(idExpiration)
			if !expireGCgo {
				go expirationGC()
				expireGCgo = true
			}
		}
		if params, found := data.getField("params"); found {
			addDataToEvent("<", params, event)
			if idFound {
				paramsByID[id] = params
			}
		}
	} else if result, found := data.getField("result"); found {
		msgType = "response"
		addDataToEvent(">", result, event)
		id, idFound := data.getField("id")
		if idFound {
			event.Any("%ID", id)
			if method, found := methodByID[id]; found {
				if method == "$/alive/listPackages" {
					addDataToEvent(">", result, event)
				}
				event.Str("<>method", method)
			}
			if params, found := paramsByID[id]; found {
				addDataToEvent("<>", params, event)
			}
		}
		if errAny, found := data.getField("error"); found && errAny != nil {
			addErrorToEvent(errAny, event)
		}
		if position, found := data.getField("position"); found {
			addToEventWithLog("position", position, event)
		}
	} else {
		msgType = "unknown"
		if str, err := marshalAny(data); err != nil {
			return fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str("msg", str)
		}
	}
	event.Str("$Type", msgType)
	event.Msg(msg)
	return nil
}

func addDataToEvent(prefix string, data any, event *zerolog.Event) {
	if hash, ok := data.(map[string]interface{}); ok {
		for key, item := range hash {
			addToEventWithLog(prefix+key, item, event)
		}
	} else if array, ok := data.([]any); ok {
		label := "rqst-params"
		if prefix == "<" {
			label = "params"
		} else if prefix == ">" {
			label = "result"
		}
		addToEventWithLog(label, array, event)
	} else if data != nil {
		log.Warn().Msg("Data not a map in addDataToEvent()")
	}
}

func addErrorToEvent(errAny any, event *zerolog.Event) {
	if errAny == nil {
		return
	} else if errHash, ok := errAny.(map[string]interface{}); ok {
		if code, found := errHash["code"]; found {
			if codeInt, ok := code.(int); ok {
				event.Int("!code", codeInt)
			}
		}
		if msg, found := errHash["message"]; found {
			if message, ok := msg.(string); ok {
				event.Str("!msg", message)
			}
		}
		if data, found := errHash["data"]; found {
			addToEventWithLog("!data", data, event)
		}
	}
}

func addToEventWithLog(label string, item any, event *zerolog.Event) {
	if found, err := addToEvent(label, item, event); err != nil {
		log.Warn().Err(err).Msgf("Adding %s to event", label)
	} else if !found {
		log.Debug().Msgf("Empty %s", label)
	}
}

func addToEvent(label string, item any, event *zerolog.Event) (bool, error) {
	added := true
	if text, ok := item.(string); ok {
		if len(text) > maxDisplayLen {
			text = text[:maxDisplayLen]
		}
		if text == "" {
			return false, nil
		} else {
			event.Str(label, text)
		}
	} else if number, ok := item.(float64); ok {
		event.Float64(label, number)
	} else if boolean, ok := item.(bool); ok {
		event.Bool(label, boolean)
	} else if hash, ok := item.(map[string]interface{}); ok && len(hash) > 0 {
		// Most useful hash data is handled in other functions,
		// just let this fall through and be shown as JSON.
		added = false
	} else if array, ok := item.([]any); ok && len(array) > 0 {
		event.Int(label+"#", len(array))
		for _, element := range array {
			if done, err := addToEvent(label+"[0]", element, event); err != nil {
				return false, fmt.Errorf("addToEvent: %w", err)
			} else if done {
				// Only shows first item in array.
				return true, nil
			}
		}
	}
	if !added {
		if str, err := marshalAny(item); err != nil {
			return false, fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str(label, str)
			added = true
		}
	}
	return true, nil
}

func marshalAny(item any) (string, error) {
	if jsonBytes, err := json.Marshal(item); err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	} else {
		if len(jsonBytes) > maxDisplayLen {
			jsonBytes = append(jsonBytes[:maxDisplayLen], []byte("...")...)
		}
		return string(jsonBytes), nil
	}
}
