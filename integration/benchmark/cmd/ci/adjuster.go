package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.einride.tech/pid"

	"github.com/onflow/flow-go/integration/benchmark"
)

type Adjuster struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}

	controller *pid.Controller
	params     AdjusterParams

	lg                 *benchmark.ContLoadGenerator
	workerStatsTracker *benchmark.WorkerStatsTracker
	log                zerolog.Logger
}
type AdjusterParams struct {
	Delay       time.Duration
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
) *Adjuster {
	ctx, cancel := context.WithCancel(ctx)
	a := &Adjuster{
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),

		controller: &pid.Controller{
			// PD controller.
			// P is needed to get to the target quickly.
			// D is needed to get there with a slower velocity.
			// I is not needed since it only adds inertia.  Our system is already an integrator
			// (i.e. the inflight transactions is an integrator of the TPS), so there is no need to add
			// an additional integrator term.
			Config: pid.ControllerConfig{
				ProportionalGain: 0.1,
				DerivativeGain:   0.1,
			},
		},

		params: params,

		lg:                 lg,
		workerStatsTracker: workerStatsTracker,
		log:                log,
	}

	go func() {
		defer close(a.done)

		log.Info().Dur("delayInMS", params.Delay).Msg("Waiting before starting TPS Adjuster")
		select {
		case <-time.After(params.Delay):
			log.Info().Msg("starting TPS Adjuster")
		case <-ctx.Done():
			return
		}

		err := a.adjustTPSForever()
		if err != nil && !errors.Is(err, context.Canceled) {
			log.Error().Err(err).Msg("Adjuster failed")
		}
	}()

	return a
}

func (a *Adjuster) Stop() {
	a.cancel()
	<-a.done
}

func (a *Adjuster) adjustTPSForever() (err error) {
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
func (a *Adjuster) adjustOnce(nowTs time.Time, lastState adjusterState) (adjusterState, error) {
	timeDiff := nowTs.Sub(lastState.timestamp)
	currentStats := a.workerStatsTracker.GetStats()

	inflight := float64(currentStats.TxsSent - currentStats.TxsExecuted)
	a.controller.Update(pid.ControllerInput{
		ReferenceSignal:  1.0,
		ActualSignal:     inflight / float64(a.params.MaxInflight),
		SamplingInterval: timeDiff,
	})
	ratio := 1. + a.controller.State.ControlSignal
	targetInflight := inflight * ratio

	unboundedTPS := uint(float64(lastState.targetTPS) * ratio)
	boundedTPS := boundTPS(unboundedTPS, a.params.MinTPS, a.params.MaxTPS)

	// number of timed out transactions in the last interval
	txsTimedout := currentStats.TxsTimedout - int(lastState.timedout)
	currentTPS := float64(currentStats.TxsExecuted-int(lastState.executed)) / timeDiff.Seconds()
	a.log.Info().
		Uint("lastTargetTPS", lastState.targetTPS).
		Float64("lastTPS", lastState.tps).
		Float64("currentTPS", currentTPS).
		Uint("unboundedTPS", unboundedTPS).
		Uint("targetTPS", boundedTPS).
		Interface("pid", a.controller.State).
		Float64("targetInflight", targetInflight).
		Float64("inflight", inflight).
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
