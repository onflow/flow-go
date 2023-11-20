// Package cruisectl implements a "cruise control" system for Flow by adjusting
// nodes' latest ProposalTiming in response to changes in the measured view rate and
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

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// TimedBlock represents a block, with a timestamp recording when the BlockTimeController received the block
type TimedBlock struct {
	Block        *model.Block
	TimeObserved time.Time // timestamp when BlockTimeController received the block, per convention in UTC
}

// epochInfo stores data about the current and next epoch. It is updated when we enter
// the first view of a new epoch, or the EpochSetup phase of the current epoch.
type epochInfo struct {
	curEpochFirstView       uint64
	curEpochFinalView       uint64 // F[v] - the final view of the current epoch
	curEpochTargetDuration  uint64
	curEpochTargetEndTime   uint64  // T[v] - the target end time of the current epoch, represented as Unix Time [seconds]
	nextEpochFinalView      *uint64 // the final view of the next epoch
	nextEpochTargetDuration *uint64
	nextEpochTargetEndTime  *uint64 // the target end time of the next epoch, represented as Unix Time [seconds]
}

// targetViewTime returns τ[v], the ideal, steady-state view time for the current epoch.
// For numerical stability, we avoid repetitive conversions between seconds and time.Duration.
// Instead, internally within the controller, we work with float64 in units of seconds.
func (epoch *epochInfo) targetViewTime() float64 {
	return float64(epoch.curEpochTargetDuration) / float64(epoch.curEpochFinalView-epoch.curEpochFirstView+1)
}

// BlockTimeController dynamically adjusts the ProposalTiming of this node,
// based on the measured view rate of the consensus committee as a whole, in
// order to achieve a desired switchover time for each epoch.
// In a nutshell, the controller outputs the block time on the happy path, i.e.
//   - Suppose the node is observing the parent block B0 at some time `x0`.
//   - The controller determines the duration `d` of how much later the child block B1
//     should be observed by the committee.
//   - The controller internally memorizes the latest B0 it has seen and outputs
//     the tuple `(B0, x0, d)`
//
// This low-level controller output `(B0, x0, d)` is wrapped into a `ProposalTiming`
// interface, specifically `happyPathBlockTime` on the happy path. The purpose of the
// `ProposalTiming` wrapper is to translate the raw controller output into a form
// that is useful for the EventHandler. Edge cases, such as initialization or
// epoch fallback are implemented by other implementations of `ProposalTiming`.
type BlockTimeController struct {
	component.Component
	protocol.Consumer // consumes protocol state events

	config *Config

	state   protocol.State
	log     zerolog.Logger
	metrics module.CruiseCtlMetrics

	epochInfo // scheduled transition view for current/next epoch
	// Currently, the only possible state transition for `epochFallbackTriggered` is false → true.
	// TODO for 'leaving Epoch Fallback via special service event' this might need to change.
	epochFallbackTriggered bool

	incorporatedBlocks chan TimedBlock   // OnBlockIncorporated events, we desire these blocks to be processed in a timely manner and therefore use a small channel capacity
	epochSetups        chan *flow.Header // EpochSetupPhaseStarted events (block header within setup phase)
	epochFallbacks     chan struct{}     // EpochFallbackTriggered events

	proportionalErr Ewma
	integralErr     LeakyIntegrator

	// latestProposalTiming holds the ProposalTiming that the controller generated in response to processing the latest observation
	latestProposalTiming *atomic.Pointer[ProposalTiming]
}

var _ hotstuff.ProposalDurationProvider = (*BlockTimeController)(nil)
var _ protocol.Consumer = (*BlockTimeController)(nil)
var _ component.Component = (*BlockTimeController)(nil)

// NewBlockTimeController returns a new BlockTimeController.
func NewBlockTimeController(log zerolog.Logger, metrics module.CruiseCtlMetrics, config *Config, state protocol.State, curView uint64) (*BlockTimeController, error) {
	// Initial error must be 0 unless we are making assumptions of the prior history of the proportional error `e[v]`
	initProptlErr, initItgErr, initDrivErr := .0, .0, .0
	proportionalErr, err := NewEwma(config.alpha(), initProptlErr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize EWMA for computing the proportional error: %w", err)
	}
	integralErr, err := NewLeakyIntegrator(config.beta(), initItgErr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LeakyIntegrator for computing the integral error: %w", err)
	}

	ctl := &BlockTimeController{
		Consumer:             events.NewNoop(),
		config:               config,
		log:                  log.With().Str("hotstuff", "cruise_ctl").Logger(),
		metrics:              metrics,
		state:                state,
		incorporatedBlocks:   make(chan TimedBlock, 3),
		epochSetups:          make(chan *flow.Header, 5),
		epochFallbacks:       make(chan struct{}, 5),
		proportionalErr:      proportionalErr,
		integralErr:          integralErr,
		latestProposalTiming: atomic.NewPointer[ProposalTiming](nil), // set in initProposalTiming
	}
	ctl.Component = component.NewComponentManagerBuilder().
		AddWorker(ctl.processEventsWorkerLogic).
		Build()

	// initialize state
	err = ctl.initEpochInfo(curView)
	if err != nil {
		return nil, fmt.Errorf("could not initialize epoch info: %w", err)
	}
	ctl.initProposalTiming(curView)

	ctl.log.Debug().
		Uint64("view", curView).
		Msg("initialized BlockTimeController")
	ctl.metrics.PIDError(initProptlErr, initItgErr, initDrivErr)
	ctl.metrics.ControllerOutput(0)
	ctl.metrics.TargetProposalDuration(0)

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

	curEpochTargetDuration, err := curEpoch.TargetDuration()
	if err != nil {
		return fmt.Errorf("could not initialize current epoch target duration: %w", err)
	}
	ctl.curEpochTargetDuration = curEpochTargetDuration

	curEpochTargetEndTime, err := curEpoch.TargetEndTime()
	if err != nil {
		return fmt.Errorf("could not initialize current epoch target end time: %w", err)
	}
	ctl.curEpochTargetEndTime = curEpochTargetEndTime

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

		nextEpochTargetDuration, err := finalSnapshot.Epochs().Next().TargetDuration()
		if err != nil {
			return fmt.Errorf("could not initialize next epoch target duration: %w", err)
		}
		ctl.nextEpochTargetDuration = &nextEpochTargetDuration

		nextEpochTargetEndTime, err := finalSnapshot.Epochs().Next().TargetEndTime()
		if err != nil {
			return fmt.Errorf("could not initialize next epoch target end time: %w", err)
		}
		ctl.nextEpochTargetEndTime = &nextEpochTargetEndTime
	}

	epochFallbackTriggered, err := ctl.state.Params().EpochFallbackTriggered()
	if err != nil {
		return fmt.Errorf("could not check epoch fallback: %w", err)
	}
	ctl.epochFallbackTriggered = epochFallbackTriggered

	return nil
}

// initProposalTiming initializes the ProposalTiming value upon startup.
// CAUTION: Must be called after initEpochInfo.
func (ctl *BlockTimeController) initProposalTiming(curView uint64) {
	// When disabled, or in epoch fallback, use fallback timing (constant ProposalDuration)
	if ctl.epochFallbackTriggered || !ctl.config.Enabled.Load() {
		ctl.storeProposalTiming(newFallbackTiming(curView, time.Now().UTC(), ctl.config.FallbackProposalDelay.Load()))
		return
	}
	// Otherwise, before we observe any view changes, publish blocks immediately
	ctl.storeProposalTiming(newPublishImmediately(curView, time.Now().UTC()))
}

// storeProposalTiming stores the latest ProposalTiming. Concurrency safe.
func (ctl *BlockTimeController) storeProposalTiming(proposalTiming ProposalTiming) {
	ctl.latestProposalTiming.Store(&proposalTiming)
}

// GetProposalTiming returns the controller's latest ProposalTiming. Concurrency safe.
func (ctl *BlockTimeController) GetProposalTiming() ProposalTiming {
	pt := ctl.latestProposalTiming.Load()
	if pt == nil { // should never happen, as we always store non-nil instances of ProposalTiming. Though, this extra check makes `GetProposalTiming` universal.
		return nil
	}
	return *pt
}

func (ctl *BlockTimeController) TargetPublicationTime(proposalView uint64, timeViewEntered time.Time, parentBlockId flow.Identifier) time.Time {
	return ctl.GetProposalTiming().TargetPublicationTime(proposalView, timeViewEntered, parentBlockId)
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
				return
			}
		default:
		}

		// Priority 2: EpochFallbackTriggered
		select {
		case <-ctl.epochFallbacks:
			err := ctl.processEpochFallbackTriggered()
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error processing epoch fallback event")
				ctx.Throw(err)
			}
		default:
		}

		// Priority 3: OnBlockIncorporated
		select {
		case <-done:
			return
		case block := <-ctl.incorporatedBlocks:
			err := ctl.processIncorporatedBlock(block)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling OnBlockIncorporated event")
				ctx.Throw(err)
				return
			}
		case block := <-ctl.epochSetups:
			snapshot := ctl.state.AtHeight(block.Height)
			err := ctl.processEpochSetupPhaseStarted(snapshot)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling EpochSetupPhaseStarted event")
				ctx.Throw(err)
				return
			}
		case <-ctl.epochFallbacks:
			err := ctl.processEpochFallbackTriggered()
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error processing epoch fallback event")
				ctx.Throw(err)
				return
			}
		}
	}
}

// processIncorporatedBlock processes `OnBlockIncorporated` events from HotStuff.
// Whenever the view changes, we:
//   - updates epoch info, if this is the first observed view of a new epoch
//   - compute error terms, compensation function output, and new ProposalTiming
//   - compute a new projected epoch end time, assuming an ideal view rate
//
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processIncorporatedBlock(tb TimedBlock) error {
	// if epoch fallback is triggered, we always use fallbackProposalTiming
	if ctl.epochFallbackTriggered {
		return nil
	}

	latest := ctl.GetProposalTiming()
	if tb.Block.View <= latest.ObservationView() { // we don't care about older blocks that are incorporated into the protocol state
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
	if ctl.nextEpochTargetEndTime == nil {
		return fmt.Errorf("cannot transition without nextEpochTargetEndTime set")
	}
	if ctl.nextEpochTargetDuration == nil {
		return fmt.Errorf("cannot transition without nextEpochTargetDuration set")
	}
	if view > *ctl.nextEpochFinalView { // the block's view should be within the upcoming epoch
		return fmt.Errorf("sanity check failed: curView %d is beyond both current epoch (final view %d) and next epoch (final view %d)",
			view, ctl.curEpochFinalView, *ctl.nextEpochFinalView)
	}

	ctl.curEpochFirstView = ctl.curEpochFinalView + 1
	ctl.curEpochFinalView = *ctl.nextEpochFinalView
	ctl.curEpochTargetDuration = *ctl.nextEpochTargetDuration
	ctl.curEpochTargetEndTime = *ctl.nextEpochTargetEndTime
	ctl.nextEpochFinalView = nil
	ctl.nextEpochTargetDuration = nil
	ctl.nextEpochTargetEndTime = nil
	return nil
}

// measureViewDuration computes a new measurement of projected epoch switchover time and error for the newly entered view.
// It updates the latest ProposalTiming based on the new error.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) measureViewDuration(tb TimedBlock) error {
	view := tb.Block.View
	// if the controller is disabled, we don't update measurements and instead use a fallback timing
	if !ctl.config.Enabled.Load() {
		fallbackDelay := ctl.config.FallbackProposalDelay.Load()
		ctl.storeProposalTiming(newFallbackTiming(view, tb.TimeObserved, fallbackDelay))
		ctl.log.Debug().
			Uint64("cur_view", view).
			Dur("fallback_proposal_delay", fallbackDelay).
			Msg("controller is disabled - using fallback timing")
		return nil
	}

	previousProposalTiming := ctl.GetProposalTiming()
	previousPropErr := ctl.proportionalErr.Value()

	// Compute the projected time still needed for the remaining views, assuming that we progress through the remaining views with
	// the idealized target view time.
	// Note the '+1' term in the computation of `viewDurationsRemaining`. This is related to our convention that the epoch begins
	// (happy path) when observing the first block of the epoch. Only by observing this block, the nodes transition to the first
	// view of the epoch. Up to that point, the consensus replicas remain in the last view of the previous epoch, in the state of
	// "having processed the last block of the old epoch and voted for it" (happy path). Replicas remain in this state until they
	// see a confirmation of the view (either QC or TC for the last view of the previous epoch).
	// In accordance with this convention, observing the proposal for the last view of an epoch, marks the start of the last view.
	// By observing the proposal, nodes enter the last view, verify the block, vote for it, the primary aggregates the votes,
	// constructs the child (for first view of new epoch). The last view of the epoch ends, when the child proposal is published.
	tau := ctl.targetViewTime()                                // τ - idealized target view time in units of seconds
	viewDurationsRemaining := ctl.curEpochFinalView + 1 - view // k[v] - views remaining in current epoch

	durationRemaining := u2t(ctl.curEpochTargetEndTime).Sub(tb.TimeObserved)

	// Compute instantaneous error term: e[v] = k[v]·τ - T[v] i.e. the projected difference from target switchover
	// and update PID controller's error terms. All UNITS in SECOND.
	instErr := float64(viewDurationsRemaining)*tau - durationRemaining.Seconds()
	propErr := ctl.proportionalErr.AddObservation(instErr)
	itgErr := ctl.integralErr.AddObservation(instErr)
	drivErr := propErr - previousPropErr

	// controller output u[v] in units of second
	u := propErr*ctl.config.KP + itgErr*ctl.config.KI + drivErr*ctl.config.KD

	// compute the controller output for this observation

	unconstrainedBlockTime := f2d(tau - u) // desired time between parent and child block, in units of seconds
	proposalTiming := newHappyPathBlockTime(tb, unconstrainedBlockTime, ctl.config.TimingConfig)
	constrainedBlockTime := proposalTiming.ConstrainedBlockTime()

	ctl.log.Debug().
		Uint64("last_observation", previousProposalTiming.ObservationView()).
		Dur("duration_since_last_observation", tb.TimeObserved.Sub(previousProposalTiming.ObservationTime())).
		Dur("projected_time_remaining", durationRemaining).
		Uint64("view_durations_remaining", viewDurationsRemaining).
		Float64("inst_err", instErr).
		Float64("proportional_err", propErr).
		Float64("integral_err", itgErr).
		Float64("derivative_err", drivErr).
		Dur("controller_output", f2d(u)).
		Dur("unconstrained_block_time", unconstrainedBlockTime).
		Dur("constrained_block_time", constrainedBlockTime).
		Msg("measured error upon view change")

	ctl.metrics.PIDError(propErr, itgErr, drivErr)
	ctl.metrics.ControllerOutput(f2d(u))
	ctl.metrics.TargetProposalDuration(proposalTiming.ConstrainedBlockTime())

	ctl.storeProposalTiming(proposalTiming)
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
		return fmt.Errorf("could not get next epoch final view: %w", err)
	}
	targetDuration, err := nextEpoch.TargetDuration()
	if err != nil {
		return fmt.Errorf("could not get next epoch target duration: %w", err)
	}
	targetEndTime, err := nextEpoch.TargetEndTime()
	if err != nil {
		return fmt.Errorf("could not get next epoch target end time: %w", err)
	}

	ctl.epochInfo.nextEpochFinalView = &finalView
	ctl.epochInfo.nextEpochTargetDuration = &targetDuration
	ctl.epochInfo.nextEpochTargetEndTime = &targetEndTime
	return nil
}

// processEpochFallbackTriggered processes EpochFallbackTriggered events from the protocol state.
// When epoch fallback mode is triggered, we:
//   - set ProposalTiming to the default value
//   - set epoch fallback triggered, to disable the controller
//
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processEpochFallbackTriggered() error {
	ctl.epochFallbackTriggered = true
	latestFinalized, err := ctl.state.Final().Head()
	if err != nil {
		return fmt.Errorf("failed to retrieve latest finalized block from protocol state %w", err)
	}

	ctl.storeProposalTiming(newFallbackTiming(latestFinalized.View, time.Now().UTC(), ctl.config.FallbackProposalDelay.Load()))
	return nil
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

// t2u converts a time.Time to UNIX time represented as a uint64.
// Returned timestamp is precise to within one second of input.
func t2u(t time.Time) uint64 {
	return uint64(t.Unix())
}

// u2t converts a UNIX timestamp represented as a uint64 to a time.Time.
func u2t(unix uint64) time.Time {
	return time.Unix(int64(unix), 0)
}

// f2d converts a floating-point number of seconds to a time.Duration.
func f2d(sec float64) time.Duration {
	return time.Duration(int64(sec * float64(time.Second)))
}
