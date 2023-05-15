package message

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/madkins23/lsp-tester/tester/data"
)

const (
	idRandomRange   = 1000
	msgHeaderFormat = "Content-Length: %d\r\n\r\n%s"
	jsonRpcVersion  = "2.0"
)

// LoadMessage loads the file at the specified path, unmarshals the JSON content,
// and returns a data.AnyMap object.
func LoadMessage(requestPath string) (data.AnyMap, error) {
	var err error
	var content []byte
	var rqst data.AnyMap
	if content, err = os.ReadFile(requestPath); err != nil {
		return nil, fmt.Errorf("read request %s: %w", requestPath, err)
	}
	if err = json.Unmarshal(content, &rqst); err != nil {
		return nil, fmt.Errorf("unmarshal request: %w", err)
	}
	return rqst, nil
}

// SendMessage marshals a data.AnyMap object and sends it to the specified connection.
// The data object is edited to contain a JSON RPC version, a request ID,
// and contained relative path fields are replaced with absolute paths.
func SendMessage(to string, message data.AnyMap, connection net.Conn, msgLgr *Logger) error {
	message["jsonrpc"] = jsonRpcVersion
	message["id"] = strconv.Itoa(idRandomRange + rand.Intn(idRandomRange))

	if params, ok := message["params"].(data.AnyMap); ok {
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
	} else if err := SendContent(to, content, connection, msgLgr); err != nil {
		return fmt.Errorf("send content: %w", err)
	}
	return nil
}

// SendContent sends byte array content to the specified connection.
// A message header is provided before the content.
func SendContent(to string, content []byte, connection net.Conn, msgLgr *Logger) error {
	msgLgr.Message("tester", to, "Send", content)
	message := fmt.Sprintf(msgHeaderFormat, len(content), string(content))
	if _, err := connection.Write([]byte(message)); err != nil {
		return fmt.Errorf("write content: %w", err)
	}

	return nil
}
