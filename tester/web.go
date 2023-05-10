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
	"strings"

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
		"messages":  messages,
		"receivers": receivers,
	}

	const configureError = "Configuring page handler"

	if err := handlePage("main", "/", data, mainPre, nil); err != nil {
		log.Error().Err(err).Str("page", "main").Msg(configureError)
	}

	if err := handlePage("send", "/send", data, nil, nil); err != nil {
		log.Error().Err(err).Str("page", "send").Msg(configureError)
	}

	sentData := make(webData)
	if err := handlePage("sent", "/sent", sentData, sendMessage, nil); err != nil {
		log.Error().Err(err).Str("page", "sent").Msg(configureError)
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
	if err := handlePage("exit", "/exit", nil, nil, func(_ *http.Request, _ webData) {
		exitChannel <- true
	}); err != nil {
		log.Error().Err(err).Str("page", "exit").Msg(configureError)
	}

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Web service failure")
	}
}

//go:embed web
var webPages embed.FS

func handlePage(name, url string, data webData, pre, post func(r *http.Request, data webData)) error {
	if page, err := webPages.ReadFile("web/" + name + ".html"); err != nil {
		return fmt.Errorf("loading web page %s: %w", name, err)
	} else if tmpl, err := template.New(name).Parse(string(page)); err != nil {
		return fmt.Errorf("template for web page %s: %w", name, err)
	} else {
		http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
			if pre != nil {
				pre(r, data)
			}
			if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
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

func mainPre(rqst *http.Request, data webData) {
	if rqst.Method != "POST" {
		return
	}
	switch rqst.FormValue("form") {
	case "send":
		sendMessage(rqst, data)
	}
}

func sendMessage(rqst *http.Request, data webData) {
	if rqst.Method != "POST" {
		data["error"] = "Wrong method: " + rqst.Method
	} else {
		log.Debug().Str("name", rqst.FormValue("name")).Msg("Form Name?")

		var errs = make([]string, 0, 2)
		var message, target string
		var rcvr *receiver
		if target = rqst.FormValue("target"); target == "" {
			errs = append(errs, "No target specified")
		} else if rcvr = receivers[target]; rcvr == nil {
			errs = append(errs, "No such receiver")
		}
		if message = rqst.FormValue("message"); message == "" {
			errs = append(errs, "No message specified")
		} else if rqst, err := loadRequest(path.Join(messageDir, message)); err != nil {
			errs = append(errs,
				fmt.Sprintf("Load request from file %s: %s", message, err))
		} else if rcvr == nil {
		} else if err := sendRequest(target, rqst, rcvr.conn); err != nil {
			errs = append(errs,
				fmt.Sprintf("Send message to server %s: %s", target, err))
		}
		if len(errs) > 0 {
			data["error"] = "<p>" + strings.Join(errs, "</p><p>") + "</p>"
		} else {
			data["result"] = "Message sent"
		}
	}
}
