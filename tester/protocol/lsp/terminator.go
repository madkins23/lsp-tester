package lsp

import (
	"errors"
	"fmt"

	"github.com/madkins23/go-utils/app"
	"github.com/rs/zerolog/log"
)

var _ app.SubSystem = (*Terminator)(nil)

type Terminator struct{}

func NewTerminator() app.SubSystem {
	return &Terminator{}
}

func (t *Terminator) Shutdown() error {
	log.Info().Str("svc", "Receivers").Msg("Shutdown")
	errs := make([]error, len(receivers))
	for key, rcvr := range receivers {
		if err := rcvr.Kill(); err != nil {
			errs = append(errs, fmt.Errorf("killing receiver %s: %w", key, err))
		}
	}
	return errors.Join(errs...)
}
