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

// BlockRateController dynamically adjusts the proposal delay of this node,
// based on the measured block rate of the consensus committee as a whole, in
// order to achieve a target overall block rate.
type BlockRateController struct {
	component.Component

	config *Config
	state  protocol.State
	log    zerolog.Logger

	lastMeasurement measurement // the most recently taken measurement
	epochInfo                   // scheduled transition view for current/next epoch

	proposalDelayMS        atomic.Float64
	epochFallbackTriggered bool

	viewChanges    chan uint64       // OnViewChange events           (view entered)
	epochSetups    chan *flow.Header // EpochSetupPhaseStarted events (block header within setup phase)
	epochFallbacks chan struct{}     // EpochFallbackTriggered events
}

// NewBlockRateController returns a new BlockRateController.
func NewBlockRateController(log zerolog.Logger, config *Config, state protocol.State, curView uint64) (*BlockRateController, error) {
	ctl := &BlockRateController{
		config:         config,
		log:            log.With().Str("component", "cruise_ctl").Logger(),
		state:          state,
		viewChanges:    make(chan uint64, 10),
		epochSetups:    make(chan *flow.Header, 5),
		epochFallbacks: make(chan struct{}, 5),
	}

	ctl.Component = component.NewComponentManagerBuilder().
		AddWorker(ctl.processEventsWorkerLogic).
		Build()

	err := ctl.initEpochInfo(curView)
	if err != nil {
		return nil, fmt.Errorf("could not initialize epoch info: %w", err)
	}
	ctl.initLastMeasurement(curView, time.Now())

	return ctl, nil
}

// initLastMeasurement initializes the lastMeasurement field.
// We set the measured view rate to the computed target view rate and the error to 0.
func (ctl *BlockRateController) initLastMeasurement(curView uint64, now time.Time) {
	viewsRemaining := float64(ctl.curEpochFinalView - curView)                                   // views remaining in current epoch
	timeRemaining := float64(ctl.epochInfo.curEpochTargetEndTime.Sub(now).Milliseconds()) / 1000 // time remaining (s) until target epoch end
	targetViewRate := viewsRemaining / timeRemaining
	ctl.lastMeasurement = measurement{
		view:            curView,
		time:            now,
		viewRate:        targetViewRate,
		aveViewRate:     targetViewRate,
		targetViewRate:  targetViewRate,
		proportionalErr: 0,
		integralErr:     0,
		derivativeErr:   0,
	}
	ctl.proposalDelayMS.Store(float64(ctl.config.DefaultProposalDelay.Milliseconds()))
}

// initEpochInfo initializes the epochInfo state upon component startup.
// No errors are expected during normal operation.
func (ctl *BlockRateController) initEpochInfo(curView uint64) error {
	finalSnapshot := ctl.state.Final()
	curEpoch := finalSnapshot.Epochs().Current()

	curEpochFirstView, err := curEpoch.FirstView()
	if err != nil {
		return fmt.Errorf("could not initialize current epoch first view: %w", err)
	}
	ctl.curEpochFirstView = curEpochFirstView

	curEpochFinalView, err := curEpoch.FinalView()
	if err != nil {
		return fmt.Errorf("could not initialize current epoch final view: %w", err)
	}
	ctl.curEpochFinalView = curEpochFinalView

	phase, err := finalSnapshot.Phase()
	if err != nil {
		return fmt.Errorf("could not check snapshot phase: %w", err)
	}
	if phase > flow.EpochPhaseStaking {
		nextEpochFinalView, err := finalSnapshot.Epochs().Next().FinalView()
		if err != nil {
			return fmt.Errorf("could not initialize next epoch final view: %w", err)
		}
		ctl.epochInfo.nextEpochFinalView = &nextEpochFinalView
	}

	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(curView, time.Now(), ctl.epochInfo)

	epochFallbackTriggered, err := ctl.state.Params().EpochFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check epoch fallback: %w", err)
	}
	ctl.epochFallbackTriggered = epochFallbackTriggered

	return nil
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
	return ctl.proposalDelayMS.Load()
}

// processEventsWorkerLogic is the logic for processing events received from other components.
// This method should be executed by a dedicated worker routine (not concurrency safe).
func (ctl *BlockRateController) processEventsWorkerLogic(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	for {

		// Priority 1: EpochSetup
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

		// Priority 2: EpochFallbackTriggered
		select {
		case <-ctl.epochFallbacks:
			ctl.processEpochFallbackTriggered()
		default:
		}

		// Priority 3: OnViewChange
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
		case <-ctl.epochFallbacks:
			ctl.processEpochFallbackTriggered()
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
	// if epoch fallback is triggered, we always use default proposal delay
	if ctl.epochFallbackTriggered {
		return nil
	}
	// duplicate events are no-ops
	if ctl.lastMeasurement.view == view {
		return nil
	}

	now := time.Now()
	err := ctl.checkForEpochTransition(view, now)
	if err != nil {
		return fmt.Errorf("could not check for epoch transition: %w", err)
	}
	err = ctl.measureViewRate(view, now)
	if err != nil {
		return fmt.Errorf("could not measure view rate: %w", err)
	}
	return nil
}

// checkForEpochTransition updates the epochInfo to reflect an epoch transition if curView
// being entered causes a transition to the next epoch. Otherwise, this is a no-op.
// No errors are expected during normal operation.
func (ctl *BlockRateController) checkForEpochTransition(curView uint64, now time.Time) error {
	if curView <= ctl.curEpochFinalView {
		// typical case - no epoch transition
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

	ctl.curEpochFirstView = ctl.curEpochFinalView + 1
	ctl.curEpochFinalView = *ctl.nextEpochFinalView
	ctl.nextEpochFinalView = nil
	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(curView, now, ctl.epochInfo)
	return nil
}

// measureViewRate computes a new measurement of view rate and error for the newly entered view.
// It updates the proposal delay based on the new error.
// No errors are expected during normal operation.
func (ctl *BlockRateController) measureViewRate(view uint64, now time.Time) error {
	lastMeasurement := ctl.lastMeasurement
	if view < lastMeasurement.view {
		return fmt.Errorf("got invalid OnViewChange event, transition from view %d to %d", lastMeasurement.view, view)
	}

	alpha := ctl.config.alpha()
	viewDiff := float64(view - lastMeasurement.view)                                             // views between current and last measurement
	timeDiff := float64(now.Sub(lastMeasurement.time).Milliseconds()) / 1000                     // time between current and last measurement
	viewsRemaining := float64(ctl.curEpochFinalView - view)                                      // views remaining in current epoch
	timeRemaining := float64(ctl.epochInfo.curEpochTargetEndTime.Sub(now).Milliseconds()) / 1000 // time remaining until target epoch end

	// compute and store the rate and error for the current view
	var nextMeasurement measurement
	nextMeasurement.view = view
	nextMeasurement.time = now
	nextMeasurement.viewRate = viewDiff / timeDiff
	nextMeasurement.aveViewRate = (alpha * nextMeasurement.viewRate) + ((1.0 - alpha) * lastMeasurement.aveViewRate)
	nextMeasurement.targetViewRate = viewsRemaining / timeRemaining
	nextMeasurement.proportionalErr = nextMeasurement.targetViewRate - nextMeasurement.aveViewRate
	nextMeasurement.integralErr = lastMeasurement.integralErr + nextMeasurement.proportionalErr
	nextMeasurement.derivativeErr = (nextMeasurement.proportionalErr - lastMeasurement.proportionalErr) / viewDiff
	ctl.lastMeasurement = nextMeasurement

	// compute and store the new proposal delay value
	delayMS := ctl.config.DefaultProposalDelayMs() +
		nextMeasurement.proportionalErr*ctl.config.KP +
		nextMeasurement.integralErr*ctl.config.KI +
		nextMeasurement.derivativeErr*ctl.config.KD
	if delayMS < ctl.config.MinProposalDelayMs() {
		ctl.proposalDelayMS.Store(ctl.config.MinProposalDelayMs())
		return nil
	}
	if delayMS > ctl.config.MaxProposalDelayMs() {
		ctl.proposalDelayMS.Store(ctl.config.MaxProposalDelayMs())
		return nil
	}
	ctl.proposalDelayMS.Store(delayMS)
	return nil
}

// processEpochSetupPhaseStarted processes EpochSetupPhaseStarted events from the protocol state.
// Whenever we enter the EpochSetup phase, we:
//   - store the next epoch's final view
//
// No errors are expected during normal operation.
func (ctl *BlockRateController) processEpochSetupPhaseStarted(snapshot protocol.Snapshot) error {
	if ctl.epochFallbackTriggered {
		return nil
	}

	nextEpoch := snapshot.Epochs().Next()
	finalView, err := nextEpoch.FinalView()
	if err != nil {
		return fmt.Errorf("could not get next epochInfo final view: %w", err)
	}
	ctl.epochInfo.nextEpochFinalView = &finalView
	return nil
}

// processEpochFallbackTriggered processes EpochFallbackTriggered events from the protocol state.
// When epoch fallback mode is triggered, we:
//   - set proposal delay to the default value
//   - set epoch fallback triggered, to disable the controller
func (ctl *BlockRateController) processEpochFallbackTriggered() {
	ctl.epochFallbackTriggered = true
	ctl.proposalDelayMS.Store(ctl.config.DefaultProposalDelayMs())
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
	ctl.epochFallbacks <- struct{}{}
}
