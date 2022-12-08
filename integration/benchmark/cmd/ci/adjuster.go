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

	// number of timed out transactions in the last interval
	txsTimedout := currentStats.TxsTimedout - int(lastState.timedout)

	inflight := currentStats.TxsSent - currentStats.TxsExecuted
	inflightPerWorker := inflight / int(lastState.targetTPS)

	skip, currentTPS, unboundedTPS := computeTPS(
		lastState.executed,
		uint(currentStats.TxsExecuted),
		lastState.timestamp,
		nowTs,
		lastState.tps,
		lastState.targetTPS,
		inflight,
		a.params.MaxInflight,
		txsTimedout > 0,
	)

	if skip {
		a.log.Info().
			Float64("lastTPS", lastState.tps).
			Float64("currentTPS", currentTPS).
			Int("inflight", inflight).
			Int("inflightPerWorker", inflightPerWorker).
			Msg("skipped adjusting TPS")

		lastState.executed = uint(currentStats.TxsExecuted)
		lastState.tps = currentTPS
		lastState.timestamp = nowTs

		return lastState, nil
	}

	boundedTPS := boundTPS(unboundedTPS, a.params.MinTPS, a.params.MaxTPS)
	a.log.Info().
		Uint("lastTargetTPS", lastState.targetTPS).
		Float64("lastTPS", lastState.tps).
		Float64("currentTPS", currentTPS).
		Uint("unboundedTPS", unboundedTPS).
		Uint("targetTPS", boundedTPS).
		Int("inflight", inflight).
		Int("inflightPerWorker", inflightPerWorker).
		Int("txsTimedout", txsTimedout).
		Msg("adjusting TPS")

	err := a.lg.SetTPS(boundedTPS)
	if err != nil {
		return lastState, fmt.Errorf("unable to set tps: %w", err)
	}

	return adjusterState{
		timestamp: nowTs,
		tps:       currentTPS,
		targetTPS: boundedTPS,

		timedout: uint(currentStats.TxsTimedout),
		executed: uint(currentStats.TxsExecuted),
	}, nil
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
