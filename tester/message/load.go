package message

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/madkins23/lsp-tester/tester/data"
	"github.com/madkins23/lsp-tester/tester/flags"
)

type Files struct {
	flags    *flags.Set
	messages []string
}

func NewFiles(flags *flags.Set) *Files {
	return &Files{
		flags: flags,
	}
}

// LoadMessageFiles loads all message files in the message directory.
// The message directory is specified by the flagSet.MessageDir() flag.
// The message file data is stored in the messages global variable as strings.
func (f *Files) LoadMessageFiles() error {
	messageDir := f.flags.MessageDir()
	if messageDir == "" {
		// Nothing to do here
	} else if entries, err := os.ReadDir(messageDir); err != nil {
		f.messages = make([]string, 0)
		return fmt.Errorf("read message directory %s: %w", messageDir, err)
	} else {
		f.messages = make([]string, 0, len(entries))
		for _, entry := range entries {
			if !entry.IsDir() {
				f.messages = append(f.messages, entry.Name())
			}
		}
	}
	return nil
}

func (f *Files) List() []string {
	return f.messages
}

///////////////////////////////////////////////////////////////////////////////

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
