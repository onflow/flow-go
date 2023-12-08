package unittest

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
)

var verbose = flag.Bool("vv", false, "print debugging logs")
var globalOnce sync.Once

func LogVerbose() {
	*verbose = true
}

// Logger returns a zerolog
// use -vv flag to print debugging logs for tests
func Logger() zerolog.Logger {
	writer := io.Discard
	if *verbose {
		writer = os.Stderr
	}

	return LoggerWithWriterAndLevel(writer, zerolog.TraceLevel)
}

func LoggerWithWriterAndLevel(writer io.Writer, level zerolog.Level) zerolog.Logger {
	globalOnce.Do(func() {
		zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	})
	log := zerolog.New(writer).Level(level).With().Timestamp().Logger()
	return log
}

// go:noinline
func LoggerForTest(t *testing.T, level zerolog.Level) zerolog.Logger {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		file = "???"
	}
	return LoggerWithLevel(level).With().
		Str("testfile", filepath.Base(file)).Str("testcase", t.Name()).Logger()
}

func LoggerWithLevel(level zerolog.Level) zerolog.Logger {
	return LoggerWithWriterAndLevel(os.Stderr, level)
}

func LoggerWithName(mod string) zerolog.Logger {
	return Logger().With().Str("module", mod).Logger()
}

func NewLoggerHook() LoggerHook {
	return LoggerHook{
		logs: &strings.Builder{},
		mu:   &sync.Mutex{},
	}
}

func HookedLogger() (zerolog.Logger, LoggerHook) {
	hook := NewLoggerHook()
	log := zerolog.New(io.Discard).Hook(hook)
	return log, hook
}

// LoggerHook implements the zerolog.Hook interface and can be used to capture
// logs for testing purposes.
type LoggerHook struct {
	logs *strings.Builder
	mu   *sync.Mutex
}

// Logs returns the logs as a string
func (hook LoggerHook) Logs() string {
	hook.mu.Lock()
	defer hook.mu.Unlock()

	return hook.logs.String()
}

// Run implements zerolog.Hook and appends the log message to the log.
func (hook LoggerHook) Run(_ *zerolog.Event, level zerolog.Level, msg string) {
	// for tests that need to test logger.Fatal(), this is useful because the parent test process will read from stdout
	// to determine if the test sub-process (that generated the logger.Fatal() call) called logger.Fatal() with the expected message
	if level == zerolog.FatalLevel {
		fmt.Println(msg)
	}

	hook.mu.Lock()
	defer hook.mu.Unlock()

	hook.logs.WriteString(msg)
}
