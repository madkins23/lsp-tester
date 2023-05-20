package message

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"

	"github.com/madkins23/lsp-tester/tester/data"
	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/logging"
)

type Logger struct {
	flags  *flags.Set
	logMgr *logging.Manager
}

func NewLogger(flagSet *flags.Set, logMgr *logging.Manager) *Logger {
	return &Logger{
		flags:  flagSet,
		logMgr: logMgr,
	}
}

func (l *Logger) Message(from, to, msg string, content []byte) {
	l.messageTo(from, to, msg, content, l.logMgr.StdLogger(), l.logMgr.StdFormat())
	if l.logMgr.HasLogFile() {
		l.messageTo(from, to, msg, content, l.logMgr.FileLogger(), l.logMgr.FileFormat())
	}
}

const (
	leftArrow  = "<--"
	rightArrow = "-->"
)

func (l *Logger) messageTo(from, to, msg string, content []byte, logger *zerolog.Logger, format string) {
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

	if format == logging.FmtKeyword {
		anyData := make(data.AnyMap)
		if err := json.Unmarshal(content, &anyData); err != nil {
			log.Warn().Err(err).Msg("Unmarshal content")
			// Fall through to end where raw JSON is added.
		} else {
			event = logger.Info().Str("!", direction).Int("#size", len(content))
			if err := l.keywordMessageFormat(anyData, event, msg); err != nil {
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

func (l *Logger) keywordMessageFormat(data data.AnyMap, event *zerolog.Event, msg string) error {
	var msgType string
	if method, found := data.GetStringField("method"); found {
		event.Str("%method", method)
		msgType = "notification"
		id, idFound := data.GetField("id")
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
		if params, found := data.GetField("params"); found {
			l.addDataToEvent("<", params, event)
			if idFound {
				paramsByID[id] = params
			}
		}
	} else if result, found := data.GetField("result"); found {
		msgType = "response"
		l.addDataToEvent(">", result, event)
		id, idFound := data.GetField("id")
		if idFound {
			event.Any("%ID", id)
			if method, found := methodByID[id]; found {
				if method == "$/alive/listPackages" {
					l.addDataToEvent(">", result, event)
				}
				event.Str("<>method", method)
			}
			if params, found := paramsByID[id]; found {
				l.addDataToEvent("<>", params, event)
			}
		}
		if errAny, found := data.GetField("error"); found && errAny != nil {
			l.addErrorToEvent(errAny, event)
		}
		if position, found := data.GetField("position"); found {
			l.addToEventWithLog("position", position, event)
		}
	} else if errAny, found := data.GetField("error"); found && errAny != nil {
		msgType = "notification"
		l.addErrorToEvent(errAny, event)
	} else {
		msgType = "unknown"
		if str, err := marshalAny(data, l.flags.MaxFieldDisplayLength()); err != nil {
			return fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str("msg", str)
		}
	}
	event.Str("$Type", msgType)
	event.Msg(msg)
	return nil
}

func (l *Logger) addDataToEvent(prefix string, data any, event *zerolog.Event) {
	if hash, ok := data.(map[string]interface{}); ok {
		for key, item := range hash {
			l.addToEventWithLog(prefix+key, item, event)
		}
	} else if array, ok := data.([]any); ok {
		label := "rqst-params"
		if prefix == "<" {
			label = "params"
		} else if prefix == ">" {
			label = "result"
		}
		l.addToEventWithLog(label, array, event)
	} else if boolean, ok := data.(bool); ok {
		event.Bool(prefix, boolean)
	} else if data != nil {
		if str, err := marshalAny(data, l.flags.MaxFieldDisplayLength()); err != nil {
			log.Warn().Err(err).Msg("Unable to marshal crap in addDataToEvent()")
		} else {
			event.Str("data", str).Msg("Data not a map")
		}
	}
}

func (l *Logger) addErrorToEvent(errAny any, event *zerolog.Event) {
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
		if anyData, found := errHash["data"]; found {
			l.addToEventWithLog("!data", anyData, event)
		}
	}
}

func (l *Logger) addToEventWithLog(label string, item any, event *zerolog.Event) {
	if found, err := l.addToEvent(label, item, event); err != nil {
		log.Warn().Err(err).Msgf("Adding %s to event", label)
	} else if !found {
		log.Debug().Msgf("Empty %s", label)
	}
}

var (
	dontTruncate = map[string]bool{
		"path": true,
		"uri":  true,
	}
	useStringField = []string{
		"uri",
	}
	subField = []string{
		"textDocument",
	}
)

func (l *Logger) addToEvent(label string, item any, event *zerolog.Event) (bool, error) {
	var added = true
	var err error
	if text, ok := item.(string); ok {
		if !strings.HasSuffix(label, "path") {
			if len(text) > l.flags.MaxFieldDisplayLength() {
				text = text[:l.flags.MaxFieldDisplayLength()]
			}
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
		if added, err = l.addHashFieldToEvent(label, hash, event); err != nil {
			return added, err
		}
	} else if array, ok := item.([]any); ok && len(array) > 0 {
		event.Int(label+"#", len(array))
		for _, element := range array {
			if done, err := l.addToEvent(label+"[0]", element, event); err != nil {
				return false, fmt.Errorf("addToEvent: %w", err)
			} else if done {
				// Only shows first item in array.
				return true, nil
			}
		}
	}
	if !added {
		if str, err := marshalAny(item, l.flags.MaxFieldDisplayLength()); err != nil {
			return false, fmt.Errorf("marshalAny: %w", err)
		} else {
			event.Str(label, str)
		}
	}
	return true, nil
}

func (l *Logger) addHashFieldToEvent(label string, hash map[string]interface{}, event *zerolog.Event) (bool, error) {
	added := false
	// Look for a useful field to replace the JSON.
	for _, fld := range useStringField {
		if field, found := hash[fld]; found {
			if fldStr, ok := field.(string); ok {
				if !dontTruncate[fld] && len(fldStr) > l.flags.MaxFieldDisplayLength() {
					fldStr = fldStr[:l.flags.MaxFieldDisplayLength()]
				}
				event.Str(label, fldStr)
				added = true
				break
			}
		}
	}
	// Look for a useful field that is a hash that can be further processed.
	for _, fld := range subField {
		if field, found := hash[fld]; found {
			var err error
			if added, err = l.addToEvent(label, field, event); err != nil {
				return false, fmt.Errorf("process sub-field: %w", err)
			}
		}
	}
	return added, nil
}

func marshalAny(item any, maxDisplayLen int) (string, error) {
	if jsonBytes, err := json.Marshal(item); err != nil {
		return "", fmt.Errorf("marshal JSON: %w", err)
	} else {
		if len(jsonBytes) > maxDisplayLen {
			jsonBytes = append(jsonBytes[:maxDisplayLen], []byte("...")...)
		}
		return string(jsonBytes), nil
	}
}
