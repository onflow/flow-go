package unittest

import (
	"flag"
	"io/ioutil"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var LogVerbose = flag.Bool("vv", false, "print debugging logs")

// Logger returns a zerolog
// use -vv flag to print debugging logs for tests
func Logger() zerolog.Logger {
	writer := ioutil.Discard

	if *LogVerbose {
		writer = os.Stderr
	}
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	log := zerolog.New(writer).Level(zerolog.DebugLevel).With().Timestamp().Logger()
	return log
}
