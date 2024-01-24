package checker

import (
	"context"
	"time"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
)

type Engine struct {
	*component.ComponentManager
	core *Core
}

// DefaultTimeInterval triggers the check once every minute,
const DefaultTimeInterval = time.Minute * 1

func NewEngine(core *Core) *Engine {
	e := &Engine{
		core: core,
	}

	e.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			err := e.runLoop(ctx, DefaultTimeInterval)
			if err != nil {
				ctx.Throw(err)
			}
		}).
		Build()

	return e
}

// runLoop runs the check every minute.
// Why using a timer instead of listening to finalized and executed events?
// because it's simpler as it doesn't need to subscribe to those events.
// It also runs less checks, note: the checker doesn't need to find the
// first mismatched block, as long as it can find a mismatch, it's good enough.
// A timer could reduce the number of checks, as it only checks once every minute.
func (e *Engine) runLoop(ctx context.Context, tickInterval time.Duration) error {
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop() // critical for ticker to be garbage collected
	for {
		select {
		case <-ticker.C:
			err := e.core.RunCheck()
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}
