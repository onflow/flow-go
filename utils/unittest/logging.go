package unittest

import (
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

var verbose = flag.Bool("vv", false, "print debugging logs")

func LogVerbose() {
	*verbose = true
}

// Logger returns a zerolog
// use -vv flag to print debugging logs for tests
func Logger() zerolog.Logger {
	writer := ioutil.Discard

	if *verbose {
		writer = os.Stderr
	}
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	return log
}

func NewLoggerHook() LoggerHook {
	return LoggerHook{
		logs: &strings.Builder{},
	}
}

func HookedLogger() (zerolog.Logger, LoggerHook) {
	hook := NewLoggerHook()
	log := zerolog.New(ioutil.Discard).Hook(hook)
	return log, hook
}

// LoggerHook implements the zerolog.Hook interface and can be used to capture
// logs for testing purposes.
type LoggerHook struct {
	logs *strings.Builder
}

// Logs returns the logs as a string
func (hook LoggerHook) Logs() string {
	return hook.logs.String()
}

// Run implements zerolog.Hook and appends the log message to the log.
func (hook LoggerHook) Run(_ *zerolog.Event, _ zerolog.Level, msg string) {
	hook.logs.WriteString(msg)
}
