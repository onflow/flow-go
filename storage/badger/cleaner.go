package badger

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/rand"
)

// Cleaner uses component.ComponentManager to implement module.Startable and module.ReadyDoneAware
// to run an internal goroutine which run badger value log garbage collection at a semi-regular interval.
// The Cleaner exists for 2 reasons:
//   - Run GC frequently enough that each GC is relatively inexpensive
//   - Avoid GC being synchronized across all nodes. Since in the happy path, all nodes have very similar
//     database load patterns, without intervention they are likely to schedule GC at the same time, which
//     can cause temporary consensus halts.
type Cleaner struct {
	component.Component
	log      zerolog.Logger
	db       *badger.DB
	metrics  module.CleanerMetrics
	ratio    float64
	interval time.Duration
}

var _ component.Component = (*Cleaner)(nil)

// NewCleaner returns a cleaner that runs the badger value log garbage collection once every `interval` duration
// if an interval of zero is passed in, we will not run the GC at all.
func NewCleaner(log zerolog.Logger, db *badger.DB, metrics module.CleanerMetrics, interval time.Duration) *Cleaner {
	// NOTE: we run garbage collection frequently at points in our business
	// logic where we are likely to have a small breather in activity; it thus
	// makes sense to run garbage collection often, with a smaller ratio, rather
	// than running it rarely and having big rewrites at once
	c := &Cleaner{
		log:      log.With().Str("component", "cleaner").Logger(),
		db:       db,
		metrics:  metrics,
		ratio:    0.2,
		interval: interval,
	}

	// Disable if passed in 0 as interval
	if c.interval == 0 {
		c.Component = &module.NoopComponent{}
		return c
	}

	c.Component = component.NewComponentManagerBuilder().
		AddWorker(c.gcWorkerRoutine).
		Build()

	return c
}

// gcWorkerRoutine runs badger GC on timely basis.
func (c *Cleaner) gcWorkerRoutine(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()
	ticker := time.NewTicker(c.nextWaitDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.runGC()

			// reset the ticker with a new interval and random jitter
			ticker.Reset(c.nextWaitDuration())
		}
	}
}

// nextWaitDuration calculates next duration for Cleaner to wait before attempting to run GC.
// We add 20% jitter into the interval, so that we don't risk nodes syncing their GC calls over time.
// Therefore GC is run every X seconds, where X is uniformly sampled from [interval, interval*1.2]
func (c *Cleaner) nextWaitDuration() time.Duration {
	jitter, err := rand.Uint64n(uint64(c.interval.Nanoseconds() / 5))
	if err != nil {
		// if randomness fails, do not use a jitter for this instance.
		// TODO: address the error properly and not swallow it.
		// In this specific case, `utils/rand` only errors if the system randomness fails
		// which is a symptom of a wider failure. Many other node components would catch such
		// a failure.
		c.log.Warn().Msg("jitter is zero beacuse system randomness has failed")
		jitter = 0
	}
	return time.Duration(c.interval.Nanoseconds() + int64(jitter))
}

// runGC runs garbage collection for badger DB, handles sentinel errors and reports metrics.
func (c *Cleaner) runGC() {
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
}
