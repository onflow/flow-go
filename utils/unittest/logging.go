package unittest

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/rs/zerolog"
)

// Logger returns a zerolog
// enable - specify whether the log will printed out, useful for debugging
//          test cases. usually set false by default to disable logging,
//          and set true when debugging.
func Logger(enable bool) zerolog.Logger {
	writer := ioutil.Discard
	if enable {
		writer = os.Stderr
	}
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	return log
}
