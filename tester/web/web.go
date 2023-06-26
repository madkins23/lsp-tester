package web

import (
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strconv"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/madkins23/go-utils/app"

	"github.com/madkins23/lsp-tester/tester/data"
	"github.com/madkins23/lsp-tester/tester/flags"
	"github.com/madkins23/lsp-tester/tester/logging"
	"github.com/madkins23/lsp-tester/tester/message"
	"github.com/madkins23/lsp-tester/tester/protocol/lsp"
	"github.com/madkins23/lsp-tester/tester/protocol/tcp"
)

type Server struct {
	flags      *flags.Set
	listener   *tcp.Listener
	logger     *zerolog.Logger
	logMgr     *logging.Manager
	msgLgr     *message.Logger
	messages   *message.Files
	terminator *app.Terminator
	waiter     *sync.WaitGroup
}

func NewWebServer(flags *flags.Set, listener *tcp.Listener, logMgr *logging.Manager,
	msgLgr *message.Logger, waiter *sync.WaitGroup, terminator *app.Terminator) *Server {
	//
	logger := log.With().Str("svc", "web").Logger()
	return &Server{
		flags:      flags,
		listener:   listener,
		logger:     &logger,
		logMgr:     logMgr,
		msgLgr:     msgLgr,
		terminator: terminator,
		waiter:     waiter,
	}
}

func (s *Server) Serve() {
	port := s.flags.WebPort()
	log.Info().Uint("port", port).Msg("Web server starting")
	defer log.Info().Uint("port", port).Msg("Web server finished")

	s.waiter.Add(1)
	defer s.waiter.Done()

	s.logger.Info().Str("URL", "http://localhost:"+strconv.Itoa(int(port))).Msg("Web service")

	if messageDir := s.flags.MessageDir(); messageDir != "" {
		s.messages = message.NewFiles(s.flags)
		if err := s.messages.LoadMessageFiles(); err != nil {
			s.logger.Warn().Err(err).Str("dir", messageDir).Msg("Unable to read message directory")
		}
	}

	anyData := data.AnyMap{
		"messages":  s.messages.List(),
		"receivers": lsp.Receivers(),
	}

	const configurePageError = "Configuring page handler"
	const configureImageError = "Configuring image handler"

	if err := s.handlePage("main", "/", anyData, s.preMain, nil); err != nil {
		s.logger.Error().Err(err).Str("page", "main").Msg(configurePageError)
	}

	for _, name := range []string{"home.png", "bomb.png"} {
		if err := s.handleImage(name); err != nil {
			s.logger.Error().Err(err).Str("image", name).Msg(configureImageError)
		}
	}

	server := &http.Server{
		Addr: ":" + strconv.Itoa(int(port)),
	}

	// Use channel and goroutine to:
	// * delay shutdown until exit page is shown and
	// * prevent web service shutdown from hanging and keeping app alive.
	exitChan := make(chan bool)
	s.terminator.Add(NewTerminator(s, server))
	go func() {
		<-exitChan
		if err := s.terminator.Shutdown(); err != nil {
			s.logger.Error().Err(err).Msg("Terminating")
		}
	}()

	if err := s.handlePage("exit", "/exit", nil, func(_ *http.Request, anyData data.AnyMap) {
		anyData["exit"] = true
	}, func(_ *http.Request, _ data.AnyMap) {
		s.logger.Info().Msg("Exit")
		exitChan <- true
	}); err != nil {
		s.logger.Error().Err(err).Str("page", "exit").Msg(configurePageError)
	}

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		s.logger.Error().Err(err).Msg("Web service failure")
	}
}

//go:embed template
var webPages embed.FS

//go:embed image
var webImages embed.FS

func (s *Server) handleImage(name string) error {
	if buf, err := webImages.ReadFile("image/" + name); err != nil {
		return fmt.Errorf("read image file %s: %w", name, err)
	} else {
		http.HandleFunc("/image/"+name, func(w http.ResponseWriter, r *http.Request) {
			name := r.URL.Path[7:]
			w.Header().Set("Content-Type", "image/png")
			if _, err := w.Write(buf); err != nil {
				s.logger.Error().Err(err).Str("image", name).Msg("Write image to HTTP response")
			}
		})
		return nil
	}
}

var (
	lastMessage string
	lastTarget  string
)

func (s *Server) handlePage(name, url string, startData data.AnyMap, pre, post func(r *http.Request, data data.AnyMap)) error {
	if tmpl, err := template.ParseFS(webPages, "template/skeleton.html", "template/"+name+".html"); err != nil {
		return fmt.Errorf("parse template files for %s: %w", name, err)
	} else {
		http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
			anyData := make(data.AnyMap)
			for key, value := range startData {
				anyData[key] = value
			}
			anyData["lastMessage"] = lastMessage
			anyData["lastTarget"] = lastTarget
			anyData["page"] = name
			anyData["stdFormat"] = data.AnyMap{
				"formatName": "Console",
				"logFormat":  s.logMgr.StdFormat(),
				"allFormats": logging.AllFormats(),
				"active":     true,
			}
			anyData["fileFormat"] = data.AnyMap{
				"formatName": "File",
				"logFormat":  s.logMgr.FileFormat(),
				"allFormats": logging.AllFormats(),
				"active":     s.logMgr.HasLogFile(),
			}

			if pre != nil {
				pre(r, anyData)
			}
			if err := tmpl.ExecuteTemplate(w, "skeleton", anyData); err != nil {
				s.logger.Error().Err(err).Str("page", name).Msg("Error serving page")
				http.Error(w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
			if post != nil {
				post(r, anyData)
			}
		})
		return nil
	}
}

func (s *Server) preMain(rqst *http.Request, data data.AnyMap) {
	if rqst.Method == "POST" {
		switch rqst.FormValue("form") {
		case "send":
			s.preSendMessagePost(rqst, data)
		case "format":
			s.preLogFormatPost(rqst, data)
		}
	}
}

func (s *Server) preLogFormatPost(rqst *http.Request, anyMap data.AnyMap) {
	formatName := rqst.FormValue("formatName")
	switch formatName {
	case "Console":
		// Assume only legal values can be returned from web page.
		s.logMgr.SetStdFormat(rqst.FormValue("logFormat"))
		if fmtData, ok := anyMap["stdFormat"].(data.AnyMap); ok {
			fmtData["logFormat"] = s.logMgr.StdFormat()
		}
		anyMap["result"] = []string{"Console log format now " + s.logMgr.StdFormat()}
	case "File":
		// Assume only legal values can be returned from web page.
		s.logMgr.SetFileFormat(rqst.FormValue("logFormat"))
		if fmtData, ok := anyMap["fileFormat"].(data.AnyMap); ok {
			fmtData["logFormat"] = s.logMgr.FileFormat()
		}
		anyMap["result"] = []string{"Log file format now " + s.logMgr.FileFormat()}
	default:
		s.logger.Error().Str("formatName", formatName).Msg("Unknown format name")
	}
}

func (s *Server) preSendMessagePost(rqst *http.Request, data data.AnyMap) {
	var errs = make([]string, 0, 2)
	var msg, tgt string
	var rcvr lsp.Receiver
	if tgt = rqst.FormValue("target"); tgt == "" {
		errs = append(errs, "No target specified")
	} else if rcvr = lsp.GetReceiver(tgt); rcvr == nil {
		errs = append(errs, "No such receiver")
	} else {
		lastTarget = tgt
		data["lastTarget"] = lastTarget
	}
	if msg = rqst.FormValue("message"); msg == "" {
		errs = append(errs, "No message specified")
	} else if rqst, err := message.LoadMessage(path.Join(s.flags.MessageDir(), msg)); err != nil {
		errs = append(errs,
			fmt.Sprintf("Load request from file %s: %s", msg, err))
	} else {
		lastMessage = msg
		data["lastMessage"] = lastMessage
		if rcvr != nil {
			if err = rcvr.SendMessage(tgt, rqst, s.msgLgr); err != nil {
				errs = append(errs,
					fmt.Sprintf("Send msg to web %s: %s", tgt, err))
			}
		}
	}
	if len(errs) > 0 {
		data["errors"] = errs
	} else {
		data["result"] = []string{"Message sent"}
	}
}
