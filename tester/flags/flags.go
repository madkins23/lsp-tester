package flags

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/madkins23/go-utils/path"
	"github.com/rs/zerolog"

	"github.com/madkins23/lsp-tester/tester/logging"
)

type Set struct {
	*flag.FlagSet
	hostAddress   string
	command       string
	commandPath   string
	commandArgs   []string
	clientPort    uint
	serverPort    uint
	webPort       uint
	messageDir    string
	requestPath   string
	maxFieldLen   uint
	logLevel      zerolog.Level
	logLevelStr   string
	logFilePath   string
	logFileAppend bool
	logFileFormat string
	logStdFormat  string
	logMsgTwice   bool
	version       bool
}

func NewSet() *Set {
	set := &Set{
		FlagSet: flag.NewFlagSet("lsp-tester", flag.ContinueOnError),
	}
	set.StringVar(&set.hostAddress, "host", "127.0.0.1", "Host address")
	set.StringVar(&set.command, "command", "", "LSP server command")
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
	set.UintVar(&set.maxFieldLen, "maxFieldLen", 32, "Maximum length of fields to display")
	set.BoolVar(&set.logMsgTwice, "logMsgTwice", false, "Log each message twice with tester in the middle")
	set.BoolVar(&set.version, "version", false, "Show lsp-tester version")
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
	var err error
	if err = s.FlagSet.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	if s.command != "" {
		parts := regexp.MustCompile("\\s+").Split(s.command, -1)
		if s.commandPath, err = exec.LookPath(parts[0]); err != nil {
			return fmt.Errorf("get path for command: %w", err)
		}
		fileInfo, err := os.Stat(s.commandPath)
		if err != nil {
			return fmt.Errorf("stat %s: %w", s.commandPath, err)
		}
		mode := fileInfo.Mode()
		if !((mode.IsRegular()) || (uint32(mode&fs.ModeSymlink) == 0)) {
			return fmt.Errorf("file %s is not normal or a symlink", s.command)
		} else if uint32(mode&0111) == 0 {
			return fmt.Errorf("file %s is not executable", s.command)
		}
		s.commandArgs = make([]string, len(parts)-1)
		for i, arg := range parts[1:] {
			if s.commandArgs[i], err = path.FixHomePath(arg); err != nil {
				return fmt.Errorf("fix home path '%s': %w", arg, err)
			}
		}
	}

	if err := s.fixMessageDirectory(); err != nil {
		return fmt.Errorf("fix message directory: %w", err)
	}

	if err := s.fixRequestPath(); err != nil {
		return fmt.Errorf("fix request path: %w", err)
	}

	if err := s.fixLogFilePath(); err != nil {
		return fmt.Errorf("fix log file path: %w", err)
	}

	var found bool
	if s.logLevel, found = logLevels[s.logLevelStr]; !found {
		return fmt.Errorf("log level '%s' does not exist", s.logLevelStr)
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

func (s *Set) HasCommand() bool {
	return s.command != ""
}

func (s *Set) Command() (string, []string) {
	return s.commandPath, s.commandArgs
}

func (s *Set) CommandArgs() string {
	return s.command
}

func (s *Set) ClientPort() uint {
	return s.clientPort
}

func (s *Set) HostAddress() string {
	return s.hostAddress
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

func (s *Set) MaxFieldDisplayLength() int {
	return int(s.maxFieldLen)
}

func (s *Set) ServerPort() int {
	return int(s.serverPort)
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

func (s *Set) LogMessageTwice() bool {
	return s.logMsgTwice
}

func (s *Set) fixLogFilePath() error {
	if s.logFilePath != "" {
		var err error
		if s.logFilePath, err = path.FixHomePath(s.logFilePath); err != nil {
			return fmt.Errorf("fix home path '%s': %w", s.logFilePath, err)
		}
		logPathDir := filepath.Dir(s.logFilePath)
		if stat, err := os.Stat(logPathDir); err != nil {
			return fmt.Errorf("verify existence of log path directory: %w", err)
		} else if !stat.IsDir() {
			return fmt.Errorf("log path directory %s not a directory", filepath.Dir(s.logFilePath))
		}
	}
	return nil
}

func (s *Set) fixMessageDirectory() error {
	if s.messageDir != "" {
		// Clean up and verify the message directory path.
		var err error
		if s.messageDir, err = path.FixHomePath(s.messageDir); err != nil {
			return fmt.Errorf("fix home path '%s': %w", s.messageDir, err)
		}
		if stat, err := os.Stat(s.messageDir); err != nil {
			return fmt.Errorf("verify existence of message directory: %w", err)
		} else if !stat.IsDir() {
			return fmt.Errorf("-messages %s not a directory", s.messageDir)
		}
	}
	return nil
}

func (s *Set) fixRequestPath() error {
	if s.requestPath != "" {
		// Clean up and verify the request path.
		// There are various possible interpretations to check.
		possiblePaths := make([]string, 0, 3)
		if strings.HasPrefix(s.requestPath, "~/") {
			if fixedPath, err := path.FixHomePath(s.requestPath); err != nil {
				return fmt.Errorf("fix home path '%s': %w", s.requestPath, err)
			} else {
				possiblePaths = append(possiblePaths, fixedPath)
			}
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
	return nil
}

func (s *Set) Version() bool {
	return s.version
}
