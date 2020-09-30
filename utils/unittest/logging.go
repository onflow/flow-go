package unittest

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var enable = false

// Logger returns a zerolog
// if go test is ran with `-v`, then the logger will print all logs,
// otherwise, the log will be hidden
func Logger() zerolog.Logger {
	writer := ioutil.Discard

	if enable {
		writer = os.Stderr
	}
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	return log
}
