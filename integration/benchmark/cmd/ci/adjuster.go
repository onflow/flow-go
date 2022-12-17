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

	params AdjusterParams

	lg                 *benchmark.ContLoadGenerator
	workerStatsTracker *benchmark.WorkerStatsTracker
	log                zerolog.Logger
}
type AdjusterParams struct {
	Interval    time.Duration
	InitialTPS  uint
	MinTPS      uint
	MaxTPS      uint
	MaxInflight uint
}

type adjusterState struct {
	timestamp time.Time
	tps       float64

	executed  uint
	timedout  uint
	targetTPS uint
}

func NewTPSAdjuster(
	ctx context.Context,
	log zerolog.Logger,
	lg *benchmark.ContLoadGenerator,
	workerStatsTracker *benchmark.WorkerStatsTracker,
	params AdjusterParams,
) *adjuster {
	ctx, cancel := context.WithCancel(ctx)
	a := &adjuster{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),

		params: params,

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

func (a *adjuster) adjustTPSForever() (err error) {
	initialStats := a.workerStatsTracker.GetStats()
	lastState := adjusterState{
		timestamp: time.Now(),
		tps:       0,
		targetTPS: a.params.InitialTPS,
		executed:  uint(initialStats.TxsExecuted),
		timedout:  uint(initialStats.TxsTimedout),
	}

	for {
		select {
		// NOTE: not using a ticker here since adjustOnce
		// can take a while and lead to uneven feedback intervals.
		case nowTs := <-time.After(a.params.Interval):
			lastState, err = a.adjustOnce(nowTs, lastState)
			if err != nil {
				return fmt.Errorf("adjusting TPS: %w", err)
			}
		case <-a.ctx.Done():
			return a.ctx.Err()
		}
	}
}

// adjustOnce tries to find the maximum TPS that the network can handle using a simple AIMD algorithm.
// The algorithm starts with minTPS as a target.  Each time it is able to reach the target TPS, it
// increases the target by `additiveIncrease`. Each time it fails to reach the target TPS, it decreases
// the target by `multiplicativeDecrease` factor.
//
// To avoid oscillation and speedup conversion we skip the adjustment stage if TPS grew
// compared to the last round.
//
// Target TPS is always bounded by [minTPS, maxTPS].
func (a *adjuster) adjustOnce(nowTs time.Time, lastState adjusterState) (adjusterState, error) {
	currentStats := a.workerStatsTracker.GetStats()

	computed := computeTPS(
		nowTs.Sub(lastState.timestamp),
		lastState,
		currentStats,
		a.params.MaxInflight,
	)
	if computed.skip {
		a.log.Info().
			Float64("lastTPS", lastState.tps).
			Float64("currentTPS", computed.currentTPS).
			Int("inflight", computed.inflight).
			Msg("skipped adjusting TPS")

		lastState.executed = uint(currentStats.TxsExecuted)
		lastState.tps = computed.currentTPS
		lastState.timestamp = nowTs

		return lastState, nil
	}

	boundedTPS := boundTPS(computed.unboundedTPS, a.params.MinTPS, a.params.MaxTPS)
	a.log.Info().
		Uint("lastTargetTPS", lastState.targetTPS).
		Float64("lastTPS", lastState.tps).
		Float64("currentTPS", computed.currentTPS).
		Uint("unboundedTPS", computed.unboundedTPS).
		Uint("targetTPS", boundedTPS).
		Int("inflight", computed.inflight).
		Int("txsTimedout", currentStats.TxsTimedout).
		Msg("adjusting TPS")

	err := a.lg.SetTPS(boundedTPS)
	if err != nil {
		return lastState, fmt.Errorf("unable to set tps: %w", err)
	}

	return adjusterState{
		timestamp: nowTs,
		tps:       computed.currentTPS,
		targetTPS: boundedTPS,

		timedout: uint(currentStats.TxsTimedout),
		executed: uint(currentStats.TxsExecuted),
	}, nil
}

type computeTPSResult struct {
	skip         bool
	currentTPS   float64
	unboundedTPS uint
	inflight     int
}

func computeTPS(
	timeDiff time.Duration,
	lastState adjusterState,
	currentStats benchmark.WorkerStats,
	maxInflight uint,
) computeTPSResult {
	if timeDiff == 0 {
		return computeTPSResult{true, 0, 0, 0}
	}

	currentTPS := float64(currentStats.TxsExecuted-int(lastState.executed)) / timeDiff.Seconds()
	unboundedTPS := uint(math.Ceil(currentTPS))

	inflight := currentStats.TxsSent - currentStats.TxsExecuted
	// If there are timed out transactions we throttle regardless of anything else.
	// We'll continue to throttle until the timed out transactions are gone.
	if currentStats.TxsTimedout > int(lastState.timedout) {
		return computeTPSResult{false, currentTPS, uint(float64(lastState.targetTPS) * multiplicativeDecrease), inflight}
	}

	// To avoid setting target TPS below current TPS,
	// we decrease the former one by the multiplicativeDecrease factor.
	//
	// This shortcut is only applicable when current inflight is less than maxInflight.
	if ((float64(unboundedTPS) >= float64(lastState.targetTPS)*multiplicativeDecrease) && (inflight < int(maxInflight))) ||
		(unboundedTPS >= lastState.targetTPS) {

		unboundedTPS = lastState.targetTPS + additiveIncrease
	} else {
		// Do not reduce the target if TPS increased since the last round.
		if (currentTPS > float64(lastState.targetTPS)) && (inflight < int(maxInflight)) {
			return computeTPSResult{true, currentTPS, lastState.targetTPS, inflight}
		}

		unboundedTPS = uint(float64(lastState.targetTPS) * multiplicativeDecrease)
	}
	return computeTPSResult{false, currentTPS, unboundedTPS, inflight}
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
