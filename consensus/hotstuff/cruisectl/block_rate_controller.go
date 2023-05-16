// Package cruisectl implements a "cruise control" system for Flow by adjusting
// nodes' block rate delay in response to changes in the measured block rate.
//
// It uses a PID controller with the block rate as the process variable and
// the set-point computed using the current view and epoch length config.
package cruisectl

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
)

// measurement represents one measurement of block rate and error.
// A measurement is taken each time the view changes for any reason.
// Each measurement measures the instantaneous and exponentially weighted
// moving average (EWMA) block rates, computes the target block rate,
// and computes the error terms.
type measurement struct {
	view            uint64    // v       - the current view
	time            time.Time // t[v]    - when we entered view v
	viewRate        float64   // r[v]    - measured instantaneous view rate at view v
	aveViewRate     float64   // r_N[v]  - EWMA block rate over past views [v-N, v]
	targetViewRate  float64   // r_SP[v] - computed target block rate at view v
	proportionalErr float64   // e_N[v]  - proportional error at view v
	integralErr     float64   // E_N[v]  - integral of error at view v
	derivativeErr   float64   // ∆_N[v]  - derivative of error at view v
}

// epochInfo stores data about the current and next epoch. It is updated when we enter
// the first view of a new epoch, or the EpochSetup phase of the current epoch.
type epochInfo struct {
	curEpochFirstView     uint64
	curEpochFinalView     uint64
	curEpochTargetEndTime time.Time
	nextEpochFinalView    *uint64
}

// BlockRateController dynamically adjusts the block rate delay of this node,
// based on the measured block rate of the consensus committee as a whole, in
// order to achieve a target overall block rate.
type BlockRateController struct {
	component.Component

	config *Config
	state  protocol.State
	log    zerolog.Logger

	lastMeasurement *measurement // the most recently taken measurement
	*epochInfo                   // scheduled transition view for current/next epoch

	proposalDelay          *atomic.Float64
	epochFallbackTriggered *atomic.Bool

	viewChanges chan uint64       // OnViewChange events           (view entered)
	epochSetups chan *flow.Header // EpochSetupPhaseStarted events (block header within setup phase)
}

// NewBlockRateController returns a new BlockRateController.
func NewBlockRateController(log zerolog.Logger, config *Config, state protocol.State) (*BlockRateController, error) {
	ctl := &BlockRateController{
		config:      config,
		log:         log.With().Str("component", "cruise_ctl").Logger(),
		state:       state,
		viewChanges: make(chan uint64, 10),
		epochSetups: make(chan *flow.Header, 5),
	}

	ctl.Component = component.NewComponentManagerBuilder().
		AddWorker(ctl.processEventsWorkerLogic).
		Build()

	// TODO initialize last measurement
	// TODO initialize epochInfo info
	_ = ctl.lastMeasurement

	return ctl, nil
}

// ProposalDelay returns the current proposal delay value to use when proposing, in milliseconds.
// This function reflects the most recently computed output of the PID controller.
// The proposal delay is the delay introduced when this node produces a block proposal,
// and is the variable adjusted by the BlockRateController to achieve a target view rate.
//
// For a given proposal, suppose the time to produce the proposal is P:
//   - if P < ProposalDelay to produce, then we wait ProposalDelay-P before broadcasting the proposal (total proposal time of ProposalDelay)
//   - if P >= ProposalDelay to produce, then we immediately broadcast the proposal (total proposal time of P)
func (ctl *BlockRateController) ProposalDelay() float64 {
	return ctl.proposalDelay.Load()
}

// processEventsWorkerLogic is the logic for processing events received from other components.
// This method should be executed by a dedicated worker routine (not concurrency safe).
func (ctl *BlockRateController) processEventsWorkerLogic(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()

	for {

		// Prioritize EpochSetup events
		select {
		case block := <-ctl.epochSetups:
			snapshot := ctl.state.AtHeight(block.Height)
			err := ctl.processEpochSetupPhaseStarted(snapshot)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling EpochSetupPhaseStarted event")
				ctx.Throw(err)
			}
		default:
		}

		select {
		case <-done:
			return
		case enteredView := <-ctl.viewChanges:
			err := ctl.processOnViewChange(enteredView)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling OnViewChange event")
				ctx.Throw(err)
			}
		case block := <-ctl.epochSetups:
			snapshot := ctl.state.AtHeight(block.Height)
			err := ctl.processEpochSetupPhaseStarted(snapshot)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling EpochSetupPhaseStarted event")
				ctx.Throw(err)
			}
		}
	}
}

// processOnViewChange processes OnViewChange events from HotStuff.
// Whenever the view changes, we:
//   - take a new measurement for instantaneous and EWMA block rate
//   - compute a new target block rate (set-point)
//   - compute error terms, compensation function output, and new block rate delay
//   - updates epoch info, if this is the first observed view of a new epoch
//
// No errors are expected during normal operation.
func (ctl *BlockRateController) processOnViewChange(view uint64) error {
	err := ctl.checkForEpochTransition(view)
	if err != nil {
		return fmt.Errorf("could not check for epoch transition: %w", err)
	}
	err = ctl.measureViewRate(view)
	if err != nil {
		return fmt.Errorf("could not measure view rate: %w", err)
	}

	return nil
}

// checkForEpochTransition updates the epochInfo to reflect an epoch transition if curView
// being entered causes a transition to the next epoch. Otherwise, this is a no-op.
// No errors are expected during normal operation.
func (ctl *BlockRateController) checkForEpochTransition(curView uint64) error {
	if curView > ctl.curEpochFinalView {
		return nil
	}
	if ctl.nextEpochFinalView == nil {
		return fmt.Errorf("cannot transition without nextEpochFinalView set")
	}
	// sanity check
	if curView > *ctl.nextEpochFinalView {
		return fmt.Errorf("sanity check failed: curView is beyond both current and next epoch (%d > %d; %d > %d)",
			curView, ctl.curEpochFinalView, curView, *ctl.nextEpochFinalView)
	}
	ctl.curEpochFinalView = *ctl.nextEpochFinalView
	ctl.nextEpochFinalView = nil
	// TODO update target end time
	return nil
}

// measureViewRate computes a new measurement for the newly entered view.
// No errors are expected during normal operation.
func (ctl *BlockRateController) measureViewRate(view uint64) error {
	now := time.Now()
	lastMeasurement := ctl.lastMeasurement
	// handle repeated events - they are a no-op
	if view == lastMeasurement.view {
		return nil
	}
	if view < lastMeasurement.view {
		return fmt.Errorf("got invalid OnViewChange event, transition from view %d to %d", lastMeasurement.view, view)
	}

	alpha := ctl.config.alpha()
	nextMeasurement := new(measurement)
	nextMeasurement.view = view
	nextMeasurement.time = now
	nextMeasurement.viewRate = ctl.computeInstantaneousViewRate(lastMeasurement.view, view, lastMeasurement.time, now)
	nextMeasurement.aveViewRate = (alpha * nextMeasurement.viewRate) + ((1.0 - alpha) * lastMeasurement.aveViewRate)
	// TODO

	return nil
}

// computeInstantaneousViewRate computes the view rate between two view measurements
// in views/second with millisecond precision.
func (ctl *BlockRateController) computeInstantaneousViewRate(v1, v2 uint64, t1, t2 time.Time) float64 {
	viewDiff := float64(v2 - v1)
	timeDiff := float64(t2.Sub(t1).Milliseconds()) * 1000
	return viewDiff / timeDiff
}

// computeTargetViewRate computes the target view rate, the set-point for the PID controller,
// in views/second with millisecond precision. The target view rate is the rate so that the
// next epoch transition will occur at the target time.
func (ctl *BlockRateController) computeTargetViewRate(curView uint64) float64 {
	viewsRemaining := float64(ctl.curEpochFinalView - curView)
	timeRemaining := float64(ctl.epochInfo.curEpochTargetEndTime.Sub(time.Now().UTC()).Milliseconds()) * 1000
	return viewsRemaining / timeRemaining
}

// processEpochSetupPhaseStarted processes EpochSetupPhaseStarted events from the protocol state.
// Whenever we enter the EpochSetup phase, we:
//   - store the next epoch's final view
//
// No errors are expected during normal operation.
func (ctl *BlockRateController) processEpochSetupPhaseStarted(snapshot protocol.Snapshot) error {
	nextEpoch := snapshot.Epochs().Next()
	finalView, err := nextEpoch.FinalView()
	if err != nil {
		return fmt.Errorf("could not get next epochInfo final view: %w", err)
	}
	ctl.epochInfo.nextEpochFinalView = &finalView
	return nil
}

// OnViewChange responds to a view-change notification from HotStuff.
// The event is queued for async processing by the worker. If the channel is full,
// the event is discarded - since we are taking an average it doesn't matter if
// occasionally miss a sample.
func (ctl *BlockRateController) OnViewChange(_, newView uint64) {
	select {
	case ctl.viewChanges <- newView:
	default:
	}
}

// EpochSetupPhaseStarted responds to the EpochSetup phase starting for the current epoch.
// The event is queued for async processing by the worker.
func (ctl *BlockRateController) EpochSetupPhaseStarted(_ uint64, first *flow.Header) {
	ctl.epochSetups <- first
}

// EpochEmergencyFallbackTriggered responds to epoch fallback mode being triggered.
func (ctl *BlockRateController) EpochEmergencyFallbackTriggered() {
	ctl.epochFallbackTriggered.Store(true)
}
