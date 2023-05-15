package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	FmtDefault = "default"
	FmtExpand  = "expand"
	FmtKeyword = "keyword"
	FmtJSON    = "json"
)

var (
	allFormats = []string{
		FmtDefault,
		FmtExpand,
		FmtKeyword,
		FmtJSON,
	}
	isFormat = map[string]bool{
		FmtDefault: true,
		FmtExpand:  true,
		FmtKeyword: true,
		FmtJSON:    true,
	}
)

func AllFormats() []string {
	return allFormats
}

func IsFormat(format string) bool {
	return isFormat[format]
}

// flagSet interface defines required functionality from a flag set.
// This allows the logging package to utilize a flags.Set object
// without importing the flags package, which uses the logging package,
// and would cause an import cycle.
type flagSet interface {
	LogLevel() zerolog.Level
	LogStdFormat() string
	LogFileFormat() string
	LogFilePath() string
	LogFileAppend() bool
}

type Manager struct {
	flags            flagSet
	plainLogger      zerolog.Logger
	stdLogger        zerolog.Logger
	fileLogger       zerolog.Logger
	stdFormat        string
	fileFormat       string
	logFile          *os.File
	logFormatWriter  map[string]*zerolog.ConsoleWriter
	fileFormatWriter map[string]*zerolog.ConsoleWriter
}

func NewManager(flags flagSet) (*Manager, error) {
	mgr := &Manager{
		flags:            flags,
		logFormatWriter:  make(map[string]*zerolog.ConsoleWriter, 2),
		fileFormatWriter: make(map[string]*zerolog.ConsoleWriter, 2),
	}

	zerolog.SetGlobalLevel(mgr.flags.LogLevel())
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}

	// Build logging infrastructure.
	mgr.plainLogger = log.Logger
	var err error
	logFileAppend := mgr.flags.LogFileAppend()
	if mgr.flags.LogFilePath() != "" {
		openFlags := os.O_CREATE | os.O_WRONLY
		if logFileAppend {
			openFlags |= os.O_APPEND
		} else {
			openFlags |= os.O_TRUNC
		}
		if mgr.logFile, err = os.OpenFile(mgr.flags.LogFilePath(), openFlags, 0666); err != nil {
			mgr.logFile = nil
			return nil, fmt.Errorf("open log file %s", mgr.flags.LogFilePath())
		} else {
			mgr.fileFormatWriter[FmtDefault] = &zerolog.ConsoleWriter{
				Out: mgr.logFile, TimeFormat: "15:04:05", NoColor: true,
			}
			mgr.fileFormatWriter[FmtExpand] = &zerolog.ConsoleWriter{
				Out: mgr.logFile, TimeFormat: "15:04:05", NoColor: true,
				FieldsExclude: []string{"msg"},
				FormatExtra:   formatMsgJSON,
			}
			if logFileAppend {
				if info, err := mgr.logFile.Stat(); err != nil {
					return nil, fmt.Errorf("stat log file: %w", err)
				} else if info.Size() > 0 {
					// Separate blocks of log statements for each run.
					_, _ = fmt.Fprintln(mgr.logFile)
				}
			}
		}
	}
	mgr.logFormatWriter[FmtDefault] = &zerolog.ConsoleWriter{
		Out: os.Stderr, TimeFormat: "15:04:05",
	}
	mgr.logFormatWriter[FmtExpand] = &zerolog.ConsoleWriter{
		Out: os.Stderr, TimeFormat: "15:04:05",
		FieldsExclude: []string{"msg"},
		FormatExtra:   formatMsgJSON,
	}

	// Configure initial formats.
	mgr.SetStdFormat(mgr.flags.LogStdFormat())
	mgr.SetFileFormat(mgr.flags.LogFileFormat())

	// Send all non-message traffic here.
	log.Logger = mgr.stdLogger
	return mgr, nil
}

func (m *Manager) Close() {
	if m.HasLogFile() {
		_ = m.logFile.Close()
	}
}

func (m *Manager) HasLogFile() bool {
	return m.logFile != nil
}

func (m *Manager) StdLogger() *zerolog.Logger {
	return &m.stdLogger
}

func (m *Manager) StdFormat() string {
	return m.stdFormat
}

func (m *Manager) SetStdFormat(format string) {
	if isFormat[format] {
		m.stdFormat = format
		switch format {
		case FmtDefault:
			fallthrough
		case FmtKeyword:
			m.stdLogger = m.plainLogger.Output(*m.logFormatWriter[FmtDefault])
		case FmtExpand:
			m.stdLogger = m.plainLogger.Output(*m.logFormatWriter[FmtExpand])
		case FmtJSON:
			m.stdLogger = m.plainLogger.Output(os.Stderr)
		default:
			log.Error().Msgf("Unknown log format: %s", format)
		}
	}
}

func (m *Manager) FileLogger() *zerolog.Logger {
	return &m.fileLogger
}

func (m *Manager) FileFormat() string {
	return m.fileFormat
}

func (m *Manager) SetFileFormat(format string) {
	if isFormat[format] {
		if m.HasLogFile() {
			m.fileFormat = format
			switch format {
			case FmtDefault:
				fallthrough
			case FmtKeyword:
				m.fileLogger = m.plainLogger.Output(*m.fileFormatWriter[FmtDefault])
			case FmtExpand:
				m.fileLogger = m.plainLogger.Output(*m.fileFormatWriter[FmtExpand])
			case FmtJSON:
				m.fileLogger = m.plainLogger.Output(m.logFile)
			default:
				log.Error().Msgf("Unknown log format: %s", format)
			}
		}
	}
}

// formatMsgJSON is a FormatExtra function for zerolog.ConsoleWriter.
// When used it formats the "msg" field of a zerolog.Event as JSON
// separately on the lines after the log entry.
func formatMsgJSON(m map[string]interface{}, buffer *bytes.Buffer) error {
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
