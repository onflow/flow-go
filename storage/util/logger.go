package util

import (
	"github.com/cockroachdb/pebble"
	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
)

// Logger implements a logger satisfying Badger's Logger interface.
type Logger struct {
	log zerolog.Logger
}

var _ pebble.Logger = (*Logger)(nil)
var _ badger.Logger = (*Logger)(nil)

func NewLogger(logger zerolog.Logger, comp string) *Logger {
	return &Logger{
		log: logger.With().Str("component", comp).Logger(),
	}
}

func (l *Logger) Fatalf(msg string, args ...interface{}) {
	l.log.Fatal().Msgf(msg, args...)
}

func (l *Logger) Errorf(msg string, args ...interface{}) {
	l.log.Error().Msgf(msg, args...)
}

func (l *Logger) Warningf(msg string, args ...interface{}) {
	l.log.Warn().Msgf(msg, args...)
}

func (l *Logger) Infof(msg string, args ...interface{}) {
	l.log.Info().Msgf(msg, args...)
}

func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.log.Debug().Msgf(msg, args...)
}
