package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/madkins23/go-utils/log"
)

//go:embed web
var webPages embed.FS

func webServer(port uint) {
	log.Info().Uint("port", port).Msg("Web server starting")
	waiter.Add(1)
	defer func() {
		log.Info().Uint("port", port).Msg("Web server finished")
		waiter.Done()
	}()

	if err := handlePage("main", "/", receivers, nil); err != nil {
		log.Error().Err(err).Str("page", "main").Msg("Configuring page handler")
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
	if err := handlePage("exit", "/exit", nil, func() {
		exitChannel <- true
	}); err != nil {
		log.Error().Err(err).Str("page", "exit").Msg("Configuring page handler")
	}

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Msg("Web service failure")
	}
}

func handlePage(name, url string, data any, action func()) error {
	if page, err := webPages.ReadFile("web/" + name + ".html"); err != nil {
		return fmt.Errorf("loading web page %s: %w", name, err)
	} else if tmpl, err := template.New(name).Parse(string(page)); err != nil {
		return fmt.Errorf("template for web page %s: %w", name, err)
	} else {
		http.HandleFunc(url, func(w http.ResponseWriter, r *http.Request) {
			if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
				log.Error().Err(err).Str("page", name).Msg("Error serving page")
				http.Error(w,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
			if action != nil {
				action()
			}
		})
		return nil
	}
}
