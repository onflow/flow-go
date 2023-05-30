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

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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

// TimedBlock represents a block, with a time stamp recording when the BlockTimeController received the block
type TimedBlock struct {
	Block        *model.Block
	TimeObserved time.Time // time stamp when BlockTimeController received the block, per convention in UTC
}

// ControllerViewDuration holds the _latest_ block observed and the duration as
// desired by the controller until the child block is released. Per convention,
// ControllerViewDuration should be treated as immutable.
type ControllerViewDuration struct {
	TimedBlock                          // latest block observed by the controller
	ChildPublicationDelay time.Duration // desired duration until releasing the child block, measured from `LatestObservedBlock.TimeObserved`
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

// fractionComplete returns the percentage of views completed of the epoch for the given curView.
// curView must be within the range [curEpochFirstView, curEpochFinalView]
// Returns the completion percentage as a float between [0, 1]
func (epoch *epochInfo) fractionComplete(curView uint64) float64 {
	return float64(curView-epoch.curEpochFirstView) / float64(epoch.curEpochFinalView-epoch.curEpochFirstView)
}

// BlockTimeController dynamically adjusts the ProposalDuration of this node,
// based on the measured view rate of the consensus committee as a whole, in
// order to achieve a desired switchover time for each epoch.
type BlockTimeController struct {
	component.Component

	config  *Config
	state   protocol.State
	log     zerolog.Logger
	metrics module.CruiseCtlMetrics

	epochInfo              // scheduled transition view for current/next epoch
	epochFallbackTriggered bool

	proposalDuration   atomic.Int64      // PID output, in nanoseconds, so it is directly convertible to time.Duration
	incorporatedBlocks chan TimedBlock   // OnBlockIncorporated events, we desire these blocks to be processed in a timely manner and therefore use a small channel capacity
	epochSetups        chan *flow.Header // EpochSetupPhaseStarted events (block header within setup phase)
	epochFallbacks     chan struct{}     // EpochFallbackTriggered events

	proportionalErr        Ewma
	integralErr            LeakyIntegrator
	latestControllerOutput atomic.Pointer[ControllerViewDuration] // CAN BE NIL
}

// NewBlockTimeController returns a new BlockTimeController.
func NewBlockTimeController(log zerolog.Logger, metrics module.CruiseCtlMetrics, config *Config, state protocol.State, curView uint64) (*BlockTimeController, error) {
	initProptlErr, initItgErr, initDrivErr := .0, .0, .0 // has to be 0 unless we are making assumptions of the prior history of the proportional error `e[v]`
	proportionalErr, err := NewEwma(config.alpha(), initProptlErr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize EWMA for computing the proportional error: %w", err)
	}
	integralErr, err := NewLeakyIntegrator(config.beta(), initItgErr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LeakyIntegrator for computing the integral error: %w", err)
	}
	ctl := &BlockTimeController{
		config:                 config,
		log:                    log.With().Str("hotstuff", "cruise_ctl").Logger(),
		metrics:                metrics,
		state:                  state,
		incorporatedBlocks:     make(chan TimedBlock),
		epochSetups:            make(chan *flow.Header, 5),
		epochFallbacks:         make(chan struct{}, 5),
		proportionalErr:        proportionalErr,
		integralErr:            integralErr,
		latestControllerOutput: atomic.Pointer[ControllerViewDuration]{},
	}

	ctl.Component = component.NewComponentManagerBuilder().
		AddWorker(ctl.processEventsWorkerLogic).
		Build()

	err = ctl.initEpochInfo(curView)
	if err != nil {
		return nil, fmt.Errorf("could not initialize epoch info: %w", err)
	}

	idealiViewTime := ctl.targetViewTime().Seconds()
	initialProposalDuration := ctl.config.DefaultProposalDuration
	ctl.proposalDuration.Store(initialProposalDuration.Nanoseconds())

	ctl.log.Debug().
		Uint64("view", curView).
		Dur("proposal_duration", initialProposalDuration).
		Msg("initialized BlockTimeController")
	ctl.metrics.PIDError(initProptlErr, initItgErr, initDrivErr)
	ctl.metrics.TargetProposalDuration(initialProposalDuration)
	ctl.metrics.ControllerOutput(0)

	return ctl, nil
}

// initEpochInfo initializes the epochInfo state upon component startup.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) initEpochInfo(curView uint64) error {
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

	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(time.Now(), ctl.epochInfo.fractionComplete(curView))

	epochFallbackTriggered, err := ctl.state.Params().EpochFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check epoch fallback: %w", err)
	}
	ctl.epochFallbackTriggered = epochFallbackTriggered

	return nil
}

// ProposalDuration returns the controller's latest view duration:
//   - ControllerViewDuration.Block represents the latest block observed by the controller
//   - ControllerViewDuration.TimeObserved is the time stamp when the controller received the block, per convention in UTC
//   - ControllerViewDuration.ChildPublicationDelay is the delay, relative to `TimeObserved`,
//     when the controller would like the child block to be published
//
// This function reflects the most recently computed output of the PID controller, where `ChildPublicationDelay`
// is adjusted by the BlockTimeController to achieve a target switchover time.
//
// For a given view where we are the leader, suppose the actual time we are done building our proposal is P:
//   - if P < TimeObserved + ChildPublicationDelay, then we wait until time stamp TimeObserved + ProposalDuration
//     to broadcast the proposal
//   - if P >= TimeObserved + ChildPublicationDelay, then we immediately broadcast the proposal
//
// Concurrency safe.
func (ctl *BlockTimeController) ProposalDuration() *ControllerViewDuration {
	return ctl.latestControllerOutput.Load()
}

// processEventsWorkerLogic is the logic for processing events received from other components.
// This method should be executed by a dedicated worker routine (not concurrency safe).
func (ctl *BlockTimeController) processEventsWorkerLogic(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
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
		case block := <-ctl.incorporatedBlocks:
			err := ctl.processIncorporatedBlock(block)
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

// processIncorporatedBlock processes `OnBlockIncorporated` events from HotStuff.
// Whenever the view changes, we:
//   - compute a new projected epoch end time, assuming an ideal view rate
//   - compute error terms, compensation function output, and new ProposalDuration
//   - updates epoch info, if this is the first observed view of a new epoch
//
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processIncorporatedBlock(tb TimedBlock) error {
	// if epoch fallback is triggered, we always use default ProposalDuration
	if ctl.epochFallbackTriggered {
		return nil
	}
	latest := ctl.latestControllerOutput.Load()
	if (latest != nil) && (tb.Block.View <= latest.Block.View) { // we don't care about older blocks that are incorporated into the protocol state
		return nil
	}

	err := ctl.checkForEpochTransition(tb)
	if err != nil {
		return fmt.Errorf("could not check for epoch transition: %w", err)
	}
	err = ctl.measureViewDuration(tb)
	if err != nil {
		return fmt.Errorf("could not measure view rate: %w", err)
	}
	return nil
}

// checkForEpochTransition updates the epochInfo to reflect an epoch transition if curView
// being entered causes a transition to the next epoch. Otherwise, this is a no-op.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) checkForEpochTransition(tb TimedBlock) error {
	view := tb.Block.View
	if view <= ctl.curEpochFinalView { // prevalent case: we are still within the current epoch
		return nil
	}

	// sanity checks, since we are beyond the final view of the most recently processed epoch:
	if ctl.nextEpochFinalView == nil { // final view of epoch we are entering should be known
		return fmt.Errorf("cannot transition without nextEpochFinalView set")
	}
	if view > *ctl.nextEpochFinalView { // the block's view should be within the upcoming epoch
		return fmt.Errorf("sanity check failed: curView %d is beyond both current epoch (final view %d) and next epoch (final view %d)",
			view, ctl.curEpochFinalView, *ctl.nextEpochFinalView)
	}

	ctl.curEpochFirstView = ctl.curEpochFinalView + 1
	ctl.curEpochFinalView = *ctl.nextEpochFinalView
	ctl.nextEpochFinalView = nil
	ctl.curEpochTargetEndTime = ctl.config.TargetTransition.inferTargetEndTime(tb.Block.Timestamp, ctl.epochInfo.fractionComplete(view))
	return nil
}

// measureViewDuration computes a new measurement of projected epoch switchover time and error for the newly entered view.
// It updates the ProposalDuration based on the new error.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) measureViewDuration(tb TimedBlock) error {
	view := tb.Block.View

	// compute the projected time still needed for the remaining views, assuming that we progress through the remaining views
	// idealized target view time:
	tau := ctl.targetViewTime().Seconds()                   // τ - idealized target view time in units of seconds
	viewsRemaining := float64(ctl.curEpochFinalView - view) // k[v] - views remaining in current epoch

	// compute instantaneous error term: e[v] = k[v]·τ - T[v] i.e. the projected difference from target switchover
	// and update PID controller's error terms
	instErr := viewsRemaining*tau - ctl.curEpochTargetEndTime.Sub(tb.Block.Timestamp).Seconds()
	previousPropErr := ctl.proportionalErr.Value()
	propErr := ctl.proportionalErr.AddObservation(instErr)
	itgErr := ctl.integralErr.AddObservation(instErr)

	// controller output u[v] in units of second
	u := propErr*ctl.config.KP + itgErr*ctl.config.KI + (propErr-previousPropErr)*ctl.config.KD
	//return time.Duration(float64(time.Second) * u)

	// compute the controller output for this measurement
	desiredViewTime := tau - u
	// constrain the proposal time according to configured boundaries
	if desiredViewTime < ctl.config.MinProposalDuration.Seconds() {
		ctl.proposalDuration.Store(ctl.config.MinProposalDuration.Nanoseconds())
		return nil
	}
	if desiredViewTime > ctl.config.MaxProposalDuration.Seconds() {
		ctl.proposalDuration.Store(ctl.config.MaxProposalDuration.Nanoseconds())
		return nil
	}
	ctl.proposalDuration.Store(int64(desiredViewTime * float64(time.Second)))
	return nil
}

// processEpochSetupPhaseStarted processes EpochSetupPhaseStarted events from the protocol state.
// Whenever we enter the EpochSetup phase, we:
//   - store the next epoch's final view
//
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processEpochSetupPhaseStarted(snapshot protocol.Snapshot) error {
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
func (ctl *BlockTimeController) processEpochFallbackTriggered() {
	ctl.epochFallbackTriggered = true
	ctl.proposalDuration.Store(ctl.config.DefaultProposalDuration.Nanoseconds())
}

// OnBlockIncorporated listens to notification from HotStuff about incorporating new blocks.
// The event is queued for async processing by the worker. If the channel is full,
// the event is discarded - since we are taking an average it doesn't matter if we
// occasionally miss a sample.
func (ctl *BlockTimeController) OnBlockIncorporated(block *model.Block) {
	select {
	case ctl.incorporatedBlocks <- TimedBlock{Block: block, TimeObserved: time.Now().UTC()}:
	default:
	}
}

// EpochSetupPhaseStarted responds to the EpochSetup phase starting for the current epoch.
// The event is queued for async processing by the worker.
func (ctl *BlockTimeController) EpochSetupPhaseStarted(_ uint64, first *flow.Header) {
	ctl.epochSetups <- first
}

// EpochEmergencyFallbackTriggered responds to epoch fallback mode being triggered.
func (ctl *BlockTimeController) EpochEmergencyFallbackTriggered() {
	ctl.epochFallbacks <- struct{}{}
}
