package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/integration/benchmark"
)

type adjuster struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	initialTPS  uint
	minTPS      uint
	maxTPS      uint
	maxInflight uint
	interval    time.Duration

	lg                 *benchmark.ContLoadGenerator
	workerStatsTracker *benchmark.WorkerStatsTracker
	log                zerolog.Logger
}

func NewTPSAdjuster(
	ctx context.Context,
	log zerolog.Logger,
	lg *benchmark.ContLoadGenerator,
	workerStatsTracker *benchmark.WorkerStatsTracker,
	interval time.Duration,
	initialTPS uint,
	minTPS uint,
	maxTPS uint,
	maxInflight uint,
) *adjuster {
	ctx, cancel := context.WithCancel(ctx)
	a := &adjuster{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),

		initialTPS:  initialTPS,
		minTPS:      minTPS,
		maxTPS:      maxTPS,
		maxInflight: maxInflight,
		interval:    interval,

		lg:                 lg,
		workerStatsTracker: workerStatsTracker,
		log:                log,
	}

	go func() {
		defer close(a.done)

		err := a.adjustTPSForever()
		if err != nil && err != context.Canceled {
			log.Error().Err(err).Msg("adjuster failed")
		}
	}()

	return a
}

func (a *adjuster) Stop() {
	a.cancel()
	<-a.done
}

// adjustTPSForever tries to find the maximum TPS that the network can handle using a simple AIMD algorithm.
// The algorithm starts with minTPS as a target.  Each time it is able to reach the target TPS, it
// increases the target by `additiveIncrease`. Each time it fails to reach the target TPS, it decreases
// the target by `multiplicativeDecrease` factor.
//
// To avoid oscillation and speedup conversion we skip the adjustment stage if TPS grew
// compared to the last round.
//
// Target TPS is always bounded by [minTPS, maxTPS].
func (a *adjuster) adjustTPSForever() error {
	targetTPS := a.initialTPS

	// Stats for the last round
	lastTs := time.Now()
	lastTPS := float64(0)
	lastStats := a.workerStatsTracker.GetStats()
	lastTxsExecuted := uint(lastStats.TxsExecuted)
	lastTxsTimedout := lastStats.TxsTimedout

	for {
		select {
		// NOTE: not using a ticker here since adjusting worker count in SetTPS
		// can take a while and lead to uneven feedback intervals.
		case nowTs := <-time.After(a.interval):
			currentStats := a.workerStatsTracker.GetStats()

			// number of timed out transactions in the last interval
			txsTimedout := currentStats.TxsTimedout - lastTxsTimedout

			inflight := currentStats.TxsSent - currentStats.TxsExecuted
			inflightPerWorker := inflight / int(targetTPS)

			skip, currentTPS, unboundedTPS := computeTPS(
				lastTxsExecuted,
				uint(currentStats.TxsExecuted),
				lastTs,
				nowTs,
				lastTPS,
				targetTPS,
				inflight,
				a.maxInflight,
				txsTimedout > 0,
			)

			if skip {
				a.log.Info().
					Float64("lastTPS", lastTPS).
					Float64("currentTPS", currentTPS).
					Int("inflight", inflight).
					Int("inflightPerWorker", inflightPerWorker).
					Msg("skipped adjusting TPS")

				lastTxsExecuted = uint(currentStats.TxsExecuted)
				lastTPS = currentTPS
				lastTs = nowTs

				continue
			}

			boundedTPS := boundTPS(unboundedTPS, a.minTPS, a.maxTPS)
			a.log.Info().
				Uint("lastTargetTPS", targetTPS).
				Float64("lastTPS", lastTPS).
				Float64("currentTPS", currentTPS).
				Uint("unboundedTPS", unboundedTPS).
				Uint("targetTPS", boundedTPS).
				Int("inflight", inflight).
				Int("inflightPerWorker", inflightPerWorker).
				Int("txsTimedout", txsTimedout).
				Msg("adjusting TPS")

			err := a.lg.SetTPS(boundedTPS)
			if err != nil {
				return fmt.Errorf("unable to set tps: %w", err)
			}

			targetTPS = boundedTPS
			lastTxsTimedout = currentStats.TxsTimedout

			// SetTPS is a blocking call, so we need to re-fetch the TxExecuted and time.
			currentStats = a.workerStatsTracker.GetStats()
			lastTxsExecuted = uint(currentStats.TxsExecuted)
			lastTPS = currentTPS
			lastTs = time.Now()
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
}

func computeTPS(
	lastTxs uint,
	currentTxs uint,
	lastTs time.Time,
	nowTs time.Time,
	lastTPS float64,
	targetTPS uint,
	inflight int,
	maxInflight uint,
	timedout bool,
) (bool, float64, uint) {
	timeDiff := nowTs.Sub(lastTs).Seconds()
	if timeDiff == 0 {
		return true, 0, 0
	}

	currentTPS := float64(currentTxs-lastTxs) / timeDiff
	unboundedTPS := uint(math.Ceil(currentTPS))

	// If there are timed out transactions we throttle regardless of anything else.
	// We'll continue to throttle until the timed out transactions are gone.
	if timedout {
		return false, currentTPS, uint(float64(targetTPS) * multiplicativeDecrease)
	}

	// To avoid setting target TPS below current TPS,
	// we decrease the former one by the multiplicativeDecrease factor.
	//
	// This shortcut is only applicable when current inflight is less than maxInflight.
	if ((float64(unboundedTPS) >= float64(targetTPS)*multiplicativeDecrease) && (inflight < int(maxInflight))) ||
		(unboundedTPS >= targetTPS) {

		unboundedTPS = targetTPS + additiveIncrease
	} else {
		// Do not reduce the target if TPS increased since the last round.
		if (currentTPS > float64(lastTPS)) && (inflight < int(maxInflight)) {
			return true, currentTPS, 0
		}

		unboundedTPS = uint(float64(targetTPS) * multiplicativeDecrease)
	}
	return false, currentTPS, unboundedTPS
}

func boundTPS(tps, min, max uint) uint {
	switch {
	case tps < min:
		return min
	case tps > max:
		return max
	default:
		return tps
	}
}
