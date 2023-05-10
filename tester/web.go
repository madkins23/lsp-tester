package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"strconv"

	"github.com/madkins23/go-utils/log"
)

type webData map[string]any

func webServer(port uint) {
	log.Info().Uint("port", port).Msg("Web server starting")
	waiter.Add(1)
	defer func() {
		log.Info().Uint("port", port).Msg("Web server finished")
		waiter.Done()
	}()

	if err := loadMessageFiles(); err != nil {
		log.Warn().Err(err).Str("dir", messageDir).Msg("Unable to read message directory")
	}

	data := webData{
		"logFormat": logFormat,
		"formats":   []string{"default", "expanded", "simple"},
		"messages":  messages,
		"receivers": receivers,
	}

	const configurePageError = "Configuring page handler"
	const configureImageError = "Configuring image handler"

	if err := handlePage("main", "/", data, preMain, nil); err != nil {
		log.Error().Err(err).Str("page", "main").Msg(configurePageError)
	}

	for _, name := range []string{"home.png", "bomb.png"} {
		if err := handleImage(name); err != nil {
			log.Error().Err(err).Str("image", name).Msg(configureImageError)
		}
	}

	server := http.Server{
		Addr: ":" + strconv.Itoa(int(port)),
	}

	exitChannel := make(chan bool)
	go func() {
		<-exitChannel
		if listener != nil {
			if err := listener.Close(); err != nil {
				log.Error().Err(err).Msg("Error killing listener")
			}
		}
		for _, rcvr := range receivers {
			if err := rcvr.kill(); err != nil {
				log.Error().Err(err).Msg("Error killing receiver")
			}
		}
		if err := server.Shutdown(context.Background()); err != nil {
			log.Error().Err(err).Msg("Error shutting down web server")
		}
	}()
	if err := handlePage("exit", "/exit", nil, func(_ *http.Request, data webData) {
		data["exit"] = true
	}, func(_ *http.Request, _ webData) {
		exitChannel <- true
	}); err != nil {
		log.Error().Err(err).Str("page", "exit").Msg(configurePageError)
	}

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Web service failure")
	}
}

//go:embed web/template
var webPages embed.FS

//go:embed web/image
var webImages embed.FS

func handleImage(name string) error {
	if buf, err := webImages.ReadFile("web/image/" + name); err != nil {
		return fmt.Errorf("read image file %s: %w", name, err)
	} else {
		http.HandleFunc("/image/"+name, func(w http.ResponseWriter, r *http.Request) {
			name := r.URL.Path[7:]
			w.Header().Set("Content-Type", "image/png")
			if _, err := w.Write(buf); err != nil {
				log.Error().Err(err).Str("image", name).Msg("Write image to HTTP response")
			}
		})
		return nil
	}
}

var (
	lastMessage string
	lastTarget  string
)

func handlePage(name, url string, startData webData, pre, post func(r *http.Request, data webData)) error {
	if tmpl, err := template.ParseFS(webPages, "web/template/skeleton.html", "web/template/"+name+".html"); err != nil {
		return fmt.Errorf("parse template files for %s: %w", name, err)
	} else {
		http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
			data := make(webData)
			for key, value := range startData {
				data[key] = value
			}
			data["lastMessage"] = lastMessage
			data["lastTarget"] = lastTarget
			data["logFormat"] = logFormat
			data["page"] = name

			if pre != nil {
				pre(r, data)
			}
			if err := tmpl.ExecuteTemplate(w, "skeleton", data); err != nil {
				log.Error().Err(err).Str("page", name).Msg("Error serving page")
				http.Error(w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
			if post != nil {
				post(r, data)
			}
		})
		return nil
	}
}

func preMain(rqst *http.Request, data webData) {
	if rqst.Method == "POST" {
		switch rqst.FormValue("form") {
		case "send":
			preSendMessage(rqst, data)
		case "format":
			preLogFormat(rqst, data)
		}
	}
}

func preLogFormat(rqst *http.Request, data webData) {
	logFormat = rqst.FormValue("logFormat")
	data["logFormat"] = logFormat
	data["result"] = []string{"Log format now " + logFormat}
}

func preSendMessage(rqst *http.Request, data webData) {
	var errs = make([]string, 0, 2)
	var msg, tgt string
	var rcvr *receiver
	if tgt = rqst.FormValue("target"); tgt == "" {
		errs = append(errs, "No target specified")
	} else if rcvr = receivers[tgt]; rcvr == nil {
		errs = append(errs, "No such receiver")
	} else {
		lastTarget = tgt
		data["lastTarget"] = lastTarget
	}
	if msg = rqst.FormValue("message"); msg == "" {
		errs = append(errs, "No message specified")
	} else if rqst, err := loadRequest(path.Join(messageDir, msg)); err != nil {
		errs = append(errs,
			fmt.Sprintf("Load request from file %s: %s", msg, err))
	} else {
		lastMessage = msg
		data["lastMessage"] = lastMessage
		if rcvr != nil {
			if err = sendRequest(tgt, rqst, rcvr.conn); err != nil {
				errs = append(errs,
					fmt.Sprintf("Send msg to server %s: %s", tgt, err))
			}
		}
	}
	if len(errs) > 0 {
		data["errors"] = errs
	} else {
		data["result"] = []string{"Message sent"}
	}
}
