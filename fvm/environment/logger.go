package environment

import "github.com/rs/zerolog"

// Logger provides access to the logger to collect logs
type LoggerProvider interface {
	Logger() zerolog.Logger
}
