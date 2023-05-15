package message

import (
	"fmt"
	"os"

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
