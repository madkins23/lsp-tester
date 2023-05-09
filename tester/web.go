package main

import (
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/madkins23/go-utils/log"
)

func webServer(port uint) {
	log.Info().Msg("Web server starting")
	waiter.Add(1)
	defer func() {
		log.Info().Msg("Web server finished")
		waiter.Done()
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
	})

	if err := http.ListenAndServe(":"+strconv.Itoa(int(port)), nil); err != nil {
		log.Error().Err(err).Msg("Web service")
	}
}
