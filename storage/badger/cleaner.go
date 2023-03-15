// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/rand"
)

type Cleaner struct {
	log     zerolog.Logger
	db      *badger.DB
	metrics module.CleanerMetrics
	enabled bool
	ratio   float64
	freq    uint
	calls   uint
}

// NewCleaner returns a cleaner that runs the badger value log garbage collection once every `frequency` calls
// if a frequency of zero is passed in, we will not run the GC at all
func NewCleaner(log zerolog.Logger, db *badger.DB, metrics module.CleanerMetrics, frequency uint) *Cleaner {
	// NOTE: we run garbage collection frequently at points in our business
	// logic where we are likely to have a small breather in activity; it thus
	// makes sense to run garbage collection often, with a smaller ratio, rather
	// than running it rarely and having big rewrites at once
	c := &Cleaner{
		log:     log.With().Str("component", "cleaner").Logger(),
		db:      db,
		metrics: metrics,
		ratio:   0.2,
		freq:    frequency,
		enabled: frequency > 0, // Disable if passed in 0 as frequency
	}
	// we don't want the entire network to run GC at the same time, so
	// distribute evenly over time
	if c.enabled {
		var err error
		c.calls, err = rand.Uintn(c.freq)
		if err != nil {
			// if true randomess in the node is broken, set `calls` to zero.
			c.calls = 0
		}
	}
	return c
}

func (c *Cleaner) RunGC() error {
	if !c.enabled {
		return nil
	}
	// only actually run approximately every frequency number of calls
	c.calls++
	if c.calls < c.freq {
		return nil
	}

	// we add 20% jitter into the interval, so that we don't risk nodes syncing
	// up on their GC calls over time
	var err error
	c.calls, err = rand.Uintn(c.freq / 5)
	if err != nil {
		return err
	}

	// run the garbage collection in own goroutine and handle sentinel errors
	go func() {
		started := time.Now()
		err := c.db.RunValueLogGC(c.ratio)
		if err == badger.ErrRejected {
			// NOTE: this happens when a GC call is already running
			c.log.Warn().Msg("garbage collection on value log already running")
			return
		}
		if err == badger.ErrNoRewrite {
			// NOTE: this happens when no files have any garbage to drop
			c.log.Debug().Msg("garbage collection on value log unnecessary")
			return
		}
		if err != nil {
			c.log.Error().Err(err).Msg("garbage collection on value log failed")
			return
		}

		runtime := time.Since(started)
		c.log.Debug().
			Dur("gc_duration", runtime).
			Msg("garbage collection on value log executed")
		c.metrics.RanGC(runtime)
	}()
	return nil
}
