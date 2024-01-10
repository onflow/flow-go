package ingest

import (
	"time"

	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"golang.org/x/time/rate"
)

const (
	cleanUpTickInterval = 5 * time.Minute
)

type RateLimiterEngine struct {
	component.Component

	limiter         *AddressRateLimiter
	cleanupInterval time.Duration
}

func NewRateLimiterEngine(limit rate.Limit) *RateLimiterEngine {
	l := &RateLimiterEngine{
		limiter:         NewAddressRateLimiter(limit),
		cleanupInterval: cleanUpTickInterval,
	}

	l.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			l.CleanupLoop(ctx)
		}).Build()
	return l
}

func (r *RateLimiterEngine) CleanupLoop(ctx irrecoverable.SignalerContext) {
	ticker := time.NewTicker(r.cleanupInterval)
	defer ticker.Stop()
	defer r.limiter.Cleanup()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.limiter.Cleanup()
		}
	}
}
