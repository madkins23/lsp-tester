package main

import (
	"fmt"
	"os"
	"time"

	"github.com/madkins23/go-utils/log"
	"github.com/rs/zerolog"
)

const (
	fmtDefault = "default"
	fmtExpand  = "expand"
	fmtKeyword = "keyword"
	fmtJSON    = "json"
)

var (
	logLevel  string
	logLevels = map[string]zerolog.Level{
		"error": zerolog.ErrorLevel,
		"warn":  zerolog.WarnLevel,
		"info":  zerolog.InfoLevel,
		"debug": zerolog.DebugLevel,
		"trace": zerolog.TraceLevel,
	}
)

var (
	stdFormat   = "default"
	fileFormat  = "default"
	logFilePath string
	fileAppend  = false
)

var (
	allFormats = []string{
		fmtDefault,
		fmtExpand,
		fmtKeyword,
		fmtJSON,
	}
	isFormat = map[string]bool{
		fmtDefault: true,
		fmtExpand:  true,
		fmtKeyword: true,
		fmtJSON:    true,
	}
)

var (
	logFormatWriter  = make(map[string]*zerolog.ConsoleWriter, 2)
	fileFormatWriter = make(map[string]*zerolog.ConsoleWriter, 2)
	logFile          *os.File
)

var (
	fileLogger  zerolog.Logger
	plainLogger zerolog.Logger
	stdLogger   zerolog.Logger
)

func setStdFormat() {
	switch stdFormat {
	case fmtDefault:
		fallthrough
	case fmtKeyword:
		stdLogger = plainLogger.Output(*logFormatWriter[fmtDefault])
	case fmtExpand:
		stdLogger = plainLogger.Output(*logFormatWriter[fmtExpand])
	case fmtJSON:
		stdLogger = plainLogger.Output(os.Stderr)
	default:
		log.Error().Msgf("Unknown log format: %s", stdFormat)
	}
	// Send all non-message traffic here.
	log.SetLogger(stdLogger)
}

func setFileFormat() {
	if logFile != nil {
		switch fileFormat {
		case fmtDefault:
			fallthrough
		case fmtKeyword:
			fileLogger = plainLogger.Output(*fileFormatWriter[fmtDefault])
		case fmtExpand:
			fileLogger = plainLogger.Output(*fileFormatWriter[fmtExpand])
		case fmtJSON:
			fileLogger = plainLogger.Output(logFile)
		default:
			log.Error().Msgf("Unknown log format: %s", fileFormat)
		}
	}
}

// logSetup preconfigures all of the possible logging mode data.
func logSetup() error {
	if level, found := logLevels[logLevel]; found {
		zerolog.SetGlobalLevel(level)
	}

	plainLogger = *log.Logger()
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().Local()
	}
	var err error
	if logFilePath != "" {
		openFlags := os.O_CREATE | os.O_WRONLY
		if fileAppend {
			openFlags |= os.O_APPEND
		} else {
			openFlags |= os.O_TRUNC
		}
		if logFile, err = os.OpenFile(logFilePath, openFlags, 0666); err != nil {
			logFile = nil
			return fmt.Errorf("open log file %s", logFilePath)
		} else {
			fileFormatWriter[fmtDefault] = &zerolog.ConsoleWriter{
				Out: logFile, TimeFormat: "15:04:05", NoColor: true,
			}
			fileFormatWriter[fmtExpand] = &zerolog.ConsoleWriter{
				Out: logFile, TimeFormat: "15:04:05", NoColor: true,
				FieldsExclude: []string{"msg"},
				FormatExtra:   formatMessageJSON,
			}
			if fileAppend {
				if info, err := logFile.Stat(); err != nil {
					return fmt.Errorf("stat log file: %w", err)
				} else if info.Size() > 0 {
					// Separate blocks of log statements for each run.
					_, _ = fmt.Fprintln(logFile)
				}
			}
		}
	}
	logFormatWriter[fmtDefault] = &zerolog.ConsoleWriter{
		Out: os.Stderr, TimeFormat: "15:04:05",
	}
	logFormatWriter[fmtExpand] = &zerolog.ConsoleWriter{
		Out: os.Stderr, TimeFormat: "15:04:05",
		FieldsExclude: []string{"msg"},
		FormatExtra:   formatMessageJSON,
	}
	return nil
}

func logShutdown() {
	if logFile != nil {
		_ = logFile.Close()
	}
}
