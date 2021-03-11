package crash

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type CleanupFunc func() error

// Crasher stores resources to attempt to clean up before crashing
type Crasher struct {
	log zerolog.Logger
	// resources to clean up before crashing
	cleanups []CleanupFunc
	// make sure we only crash once
	once sync.Once
}

func New(log zerolog.Logger) *Crasher {
	crasher := &Crasher{
		log: log,
	}
	return crasher
}

func (c *Crasher) WithCleanup(cleanup CleanupFunc) *Crasher {
	c.cleanups = append(c.cleanups, cleanup)
	return c
}

func (c *Crasher) Crash() {
	c.once.Do(c.crash)
}

func (c *Crasher) crash() {

	// in case a cleanup blocks, definitely crash after 1 minute
	go func() {
		time.Sleep(time.Second * 60)
		c.log.Fatal().Msg("timed out while cleaning up - crashing")
	}()

	// execute each queued cleanup
	for _, cleanup := range c.cleanups {
		err := cleanup()
		if err != nil {
			c.log.Error().Err(err).Msg("failed to clean up resource")
		}
	}
	c.log.Fatal().Msg("crashing")
}

var (
	singleton *Crasher
	once      sync.Once
)

func Set(c *Crasher) {
	once.Do(func() {
		singleton = c
	})
}

func Crash() {
	singleton.Crash()
}
