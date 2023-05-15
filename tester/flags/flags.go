package flags

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/madkins23/lsp-tester/tester/logging"
)

type Set struct {
	*flag.FlagSet
	hostAddress   string
	clientPort    uint
	serverPort    uint
	webPort       uint
	messageDir    string
	requestPath   string
	logLevel      zerolog.Level
	logLevelStr   string
	logFilePath   string
	logFileAppend bool
	logFileFormat string
	logStdFormat  string
}

func NewSet() *Set {
	set := &Set{
		FlagSet: flag.NewFlagSet("lsp-tester", flag.ContinueOnError),
	}
	set.StringVar(&set.hostAddress, "host", "127.0.0.1", "Host address")
	set.UintVar(&set.clientPort, "clientPort", 0, "Port number served for extension to contact")
	set.UintVar(&set.serverPort, "serverPort", 0, "Port number on which to contact LSP server")
	set.UintVar(&set.webPort, "webPort", 0, "Web port number to enable web access")
	set.StringVar(&set.messageDir, "messages", "", "Path to directory of message files")
	set.StringVar(&set.requestPath, "request", "", "Path to requestPath file (client mode)")
	set.StringVar(&set.logLevelStr, "logLevel", "info", "Set log level")
	set.StringVar(&set.logStdFormat, "logFormat", logging.FmtDefault, "Console output format")
	set.StringVar(&set.logFilePath, "logFile", "", "Log file path")
	set.BoolVar(&set.logFileAppend, "fileAppend", false, "Append to any pre-existing log file")
	set.StringVar(&set.logFileFormat, "fileFormat", logging.FmtDefault, "Log file format")
	return set
}

var logLevels = map[string]zerolog.Level{
	"error": zerolog.ErrorLevel,
	"warn":  zerolog.WarnLevel,
	"info":  zerolog.InfoLevel,
	"debug": zerolog.DebugLevel,
	"trace": zerolog.TraceLevel,
}

func (s *Set) Parse(args []string) error {
	if err := s.FlagSet.Parse(args); err != nil {
		return err
	}

	if s.messageDir != "" {
		// Clean up and verify the message directory path.
		if strings.HasPrefix(s.messageDir, "~/") {
			dirname, _ := os.UserHomeDir()
			s.messageDir = filepath.Join(dirname, s.messageDir[2:])
		}
		if stat, err := os.Stat(s.messageDir); err != nil {
			return fmt.Errorf("verify existence of message directory: %w", err)
		} else if !stat.IsDir() {
			return fmt.Errorf("-messages %s not a directory", s.messageDir)
		}
	}

	if s.requestPath != "" {
		// Clean up and verify the request path.
		// There are various possible interpretations to check.
		possiblePaths := make([]string, 0, 3)
		if strings.HasPrefix(s.requestPath, "~/") {
			// Only one possible path in the tilde case.
			dirname, _ := os.UserHomeDir()
			possiblePaths = append(possiblePaths, filepath.Join(dirname, s.requestPath[2:]))
		} else {
			// Request path might be relative to messages path.
			if s.messageDir != "" {
				possiblePaths = append(possiblePaths, filepath.Join(s.messageDir, s.requestPath))
			}
			// Request path might expand to absolute path,
			// if not we don't care about the actual error.
			if abs, err := filepath.Abs(s.requestPath); err == nil {
				possiblePaths = append(possiblePaths, abs)
			}
			// Request path as provided might be just right.
			possiblePaths = append(possiblePaths, s.requestPath)
		}
		// Take the first possible path that exist and is not a directory.
		var found bool
		for _, possiblePath := range possiblePaths {
			if stat, err := os.Stat(possiblePath); err == nil && !stat.IsDir() {
				s.requestPath = possiblePath
				found = true
			}
		}
		if !found {
			return fmt.Errorf("request path %s not found", s.requestPath)
		}
	}

	var found bool
	if s.logLevel, found = logLevels[s.logLevelStr]; !found {
		return fmt.Errorf("log level '%s' does not exist", s.logLevelStr)
	}

	if s.logFilePath != "" {
		logPathDir := filepath.Dir(s.logFilePath)
		if stat, err := os.Stat(logPathDir); err != nil {
			return fmt.Errorf("verify existence of log path directory: %w", err)
		} else if !stat.IsDir() {
			return fmt.Errorf("log path directory %s not a directory", filepath.Dir(s.logFilePath))
		}
	}

	formatErrors := make([]error, 0, 2)
	if !logging.IsFormat(s.logStdFormat) {
		formatErrors = append(formatErrors, fmt.Errorf("unrecognized -logFormat=%s", s.logStdFormat))
	}
	if s.logFilePath != "" && !logging.IsFormat(s.logFileFormat) {
		formatErrors = append(formatErrors, fmt.Errorf("unrecognized -fileFormat=%s", s.logFileFormat))
	}
	if len(formatErrors) > 0 {
		return errors.Join(formatErrors...)
	}

	return nil
}

func (s *Set) HostAddress() string {
	return s.hostAddress
}

func (s *Set) ClientPort() uint {
	return s.clientPort
}

func (s *Set) LogFileAppend() bool {
	return s.logFileAppend
}

func (s *Set) LogFilePath() string {
	return s.logFilePath
}

func (s *Set) LogFileFormat() string {
	return s.logFileFormat
}

func (s *Set) LogStdFormat() string {
	return s.logStdFormat
}

func (s *Set) LogLevel() zerolog.Level {
	return s.logLevel
}

func (s *Set) ServerPort() uint {
	return s.serverPort
}

func (s *Set) WebPort() uint {
	return s.webPort
}

func (s *Set) MessageDir() string {
	return s.messageDir
}

func (s *Set) RequestPath() string {
	return s.requestPath
}
