// Package cruisectl implements a "cruise control" system for Flow by adjusting
// nodes' ProposalDuration in response to changes in the measured view rate and
// target epoch switchover time.
//
// It uses a PID controller with the projected epoch switchover time as the process
// variable and the set-point computed using epoch length config. The error is
// the difference between the projected epoch switchover time, assuming an
// ideal view time τ, and the target epoch switchover time (based on a schedule).
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

// measurement represents a measurement of error associated with entering view v.
// A measurement is taken each time the view changes for any reason.
// Each measurement computes the instantaneous error `e[v]` based on the projected
// and target epoch switchover times, and updates error terms.
type measurement struct {
	view            uint64    // v       - the current view
	time            time.Time // t[v]    - when we entered view v
	instErr         float64   // e[v]    - instantaneous error at view v (seconds)
	proportionalErr float64   // e_N[v]  - proportional error at view v  (seconds)
	integralErr     float64   // I_M[v]  - integral of error at view v   (seconds)
	derivativeErr   float64   // ∆_N[v]  - derivative of error at view v (seconds)

	// informational fields - not required for controller operation
	viewDiff uint64        // number of views since the previous measurement
	viewTime time.Duration // time since the last measurement
}

// epochInfo stores data about the current and next epoch. It is updated when we enter
// the first view of a new epoch, or the EpochSetup phase of the current epoch.
type epochInfo struct {
	curEpochFirstView     uint64
	curEpochFinalView     uint64    // F[v] - the final view of the epoch
	curEpochTargetEndTime time.Time // T[v] - the target end time of the current epoch
	nextEpochFinalView    *uint64
}

// targetViewTime returns τ[v], the ideal, steady-state view time for the current epoch.
func (epoch *epochInfo) targetViewTime() time.Duration {
	return time.Duration(float64(epochLength) / float64(epoch.curEpochFinalView-epoch.curEpochFirstView+1))
}

// pctComplete returns the percentage of views completed of the epoch for the given curView.
// curView must be within the range [curEpochFirstView, curEpochFinalView]
// Returns the completion percentage as a float between [0, 1]
func (epoch *epochInfo) pctComplete(curView uint64) float64 {
	return float64(curView-epoch.curEpochFirstView) / float64(epoch.curEpochFinalView-epoch.curEpochFirstView)
}

// BlockRateController dynamically adjusts the ProposalDuration of this node,
// based on the measured view rate of the consensus committee as a whole, in
// order to achieve a desired switchover time for each epoch.
type BlockRateController struct {
	component.Component

	config *Config
	state  protocol.State
	log    zerolog.Logger

	lastMeasurement measurement // the most recently taken measurement
	epochInfo                   // scheduled transition view for current/next epoch

	proposalDuration       atomic.Int64 // PID output, in nanoseconds, so it is directly convertible to time.Duration
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
	ctl.lastMeasurement = measurement{
		view:            curView,
		time:            now,
		proportionalErr: 0,
		integralErr:     0,
		derivativeErr:   0,
	}
	ctl.proposalDuration.Store(ctl.targetViewTime().Nanoseconds())
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

	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(time.Now(), ctl.epochInfo.pctComplete(curView))

	epochFallbackTriggered, err := ctl.state.Params().EpochFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check epoch fallback: %w", err)
	}
	ctl.epochFallbackTriggered = epochFallbackTriggered

	return nil
}

// ProposalDuration returns the current ProposalDuration value to use when proposing.
// This function reflects the most recently computed output of the PID controller.
// The ProposalDuration is the total time it takes for this node to produce a block proposal,
// from the time we enter a view to when we transmit the proposal to the committee.
// It is the variable adjusted by the BlockRateController to achieve a target switchover time.
//
// For a given view where we are the leader, suppose the actual time taken to build our proposal is P:
//   - if P < ProposalDuration, then we wait ProposalDuration-P before broadcasting the proposal (total proposal time of ProposalDuration)
//   - if P >= ProposalDuration, then we immediately broadcast the proposal (total proposal time of P)
func (ctl *BlockRateController) ProposalDuration() time.Duration {
	return time.Duration(ctl.proposalDuration.Load())
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
//   - compute a new projected epoch end time, assuming an ideal view rate
//   - compute error terms, compensation function output, and new ProposalDuration
//   - updates epoch info, if this is the first observed view of a new epoch
//
// No errors are expected during normal operation.
func (ctl *BlockRateController) processOnViewChange(view uint64) error {
	// if epoch fallback is triggered, we always use default ProposalDuration
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
	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(now, ctl.epochInfo.pctComplete(curView))
	return nil
}

// measureViewRate computes a new measurement of projected epoch switchover time and error for the newly entered view.
// It updates the ProposalDuration based on the new error.
// No errors are expected during normal operation.
func (ctl *BlockRateController) measureViewRate(view uint64, now time.Time) error {
	lastMeasurement := ctl.lastMeasurement
	if view < lastMeasurement.view {
		return fmt.Errorf("got invalid OnViewChange event, transition from view %d to %d", lastMeasurement.view, view)
	}

	alpha := ctl.config.alpha()                             // α    - inclusion parameter for error EWMA
	beta := ctl.config.beta()                               // ß    - memory parameter for error integration
	viewsRemaining := float64(ctl.curEpochFinalView - view) // k[v] - views remaining in current epoch

	var curMeasurement measurement
	curMeasurement.view = view
	curMeasurement.time = now
	curMeasurement.viewTime = now.Sub(lastMeasurement.time) // time since the last measurement
	curMeasurement.viewDiff = view - lastMeasurement.view   // views since the last measurement

	// γ[v] = k[v]•τ - the projected time remaining in the epoch
	estTimeRemaining := time.Duration(viewsRemaining * float64(ctl.targetViewTime()))
	// e[v] = t[v]+γ-T[v] - the projected difference from target switchover
	curMeasurement.instErr = now.Add(estTimeRemaining).Sub(ctl.curEpochTargetEndTime).Seconds()

	// e_N[v] = α•e[v] + (1-α)e_N[v-1]
	curMeasurement.proportionalErr = alpha*curMeasurement.instErr + (1.0-alpha)*lastMeasurement.proportionalErr
	// I_M[v] = e[v] + (1-ß)I_M[v-1]
	curMeasurement.integralErr = curMeasurement.instErr + (1.0-beta)*lastMeasurement.integralErr
	// ∆_N[v] = e_N[v] - e_n[v-1]
	curMeasurement.derivativeErr = (curMeasurement.proportionalErr - lastMeasurement.proportionalErr) / float64(curMeasurement.viewDiff)
	ctl.lastMeasurement = curMeasurement

	// compute the controller output for this measurement
	proposalTime := ctl.targetViewTime() - ctl.controllerOutput()
	// constrain the proposal time according to configured boundaries
	if proposalTime < ctl.config.MinProposalDuration {
		ctl.proposalDuration.Store(ctl.config.MinProposalDuration.Nanoseconds())
		return nil
	}
	if proposalTime > ctl.config.MaxProposalDuration {
		ctl.proposalDuration.Store(ctl.config.MaxProposalDuration.Nanoseconds())
		return nil
	}
	ctl.proposalDuration.Store(proposalTime.Nanoseconds())
	return nil
}

// controllerOutput returns u[v], the output of the controller for the most recent measurement.
// It represents the amount of time by which the controller wishes to deviate from the ideal view duration τ[v].
// Then, the ProposalDuration is given by:
//
//	τ[v]-u[v]
func (ctl *BlockRateController) controllerOutput() time.Duration {
	curMeasurement := ctl.lastMeasurement
	u := curMeasurement.proportionalErr*ctl.config.KP +
		curMeasurement.integralErr*ctl.config.KI +
		curMeasurement.derivativeErr*ctl.config.KD
	return time.Duration(float64(time.Second) * u)
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
//   - set ProposalDuration to the default value
//   - set epoch fallback triggered, to disable the controller
func (ctl *BlockRateController) processEpochFallbackTriggered() {
	ctl.epochFallbackTriggered = true
	ctl.proposalDuration.Store(ctl.config.DefaultProposalDuration.Nanoseconds())
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
