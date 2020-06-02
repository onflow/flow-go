package util

import (
	"github.com/rs/zerolog"
)

// Logger implements a logger satisfying Badger's Logger interface.
type Logger struct {
	log zerolog.Logger
}

func NewLogger(logger zerolog.Logger) *Logger {
	return &Logger{
		log: logger.With().Str("component", "badger").Logger(),
	}
}

func (l *Logger) Errorf(msg string, args ...interface{}) {
	l.log.Error().Msgf(msg, args)
}

func (l *Logger) Warningf(msg string, args ...interface{}) {
	l.log.Warn().Msgf(msg, args)
}

func (l *Logger) Infof(msg string, args ...interface{}) {
	l.log.Info().Msgf(msg, args)
}

func (l *Logger) Debugf(msg string, args ...interface{}) {
	l.log.Debug().Msgf(msg, args)
}
