package crash

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type CleanupFunc func() error

type Crasher struct {
	log      zerolog.Logger
	cleanups []CleanupFunc
}

func (c *Crasher) Crash() {
	for _, cleanup := range c.cleanups {
		err := cleanup()
		if err != nil {
			log.Error().Err(err).Msg("failed to close")
		}
	}
	log.Fatal().Msg("")
}

var defaultCrasher *Crasher
