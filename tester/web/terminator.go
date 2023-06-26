package web

import (
	"context"
	"fmt"
	"net/http"

	"github.com/madkins23/go-utils/app"
)

var _ app.SubSystem = (*Terminator)(nil)

type Terminator struct {
	http *http.Server
	web  *Server
}

func NewTerminator(webServer *Server, httpServer *http.Server) app.SubSystem {
	return &Terminator{
		http: httpServer,
		web:  webServer,
	}
}

func (t *Terminator) Shutdown() error {
	t.web.logger.Info().Msg("Shutdown")
	if t.web.listener != nil {
		t.web.listener.Close()
	}
	if err := t.http.Shutdown(context.Background()); err != nil {
		return fmt.Errorf("shut down HTTP server: %w", err)
	}

	return nil
}
