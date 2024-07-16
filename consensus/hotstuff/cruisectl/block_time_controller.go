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

// epochTiming encapsulates the timing information of one specific epoch:
type epochTiming struct {
	firstView      uint64 // first view of the epoch's view range
	finalView      uint64 // last view of the epoch's view range
	targetDuration uint64 // desired total duration of the epoch in seconds
	targetEndTime  uint64 // target end time of the epoch, represented as Unix Time [seconds]
}

// newEpochTiming queries the timing information from the given `epoch` and returns it as a new `epochTiming` instance.
func newEpochTiming(epoch protocol.Epoch) (*epochTiming, error) {
	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch's first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch's final view: %w", err)
	}
	targetDuration, err := epoch.TargetDuration()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch's target duration: %w", err)
	}
	targetEndTime, err := epoch.TargetEndTime()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch's target end time: %w", err)
	}

	return &epochTiming{
		firstView:      firstView,
		finalView:      finalView,
		targetDuration: targetDuration,
		targetEndTime:  targetEndTime,
	}, nil
}

// targetViewTime returns τ[v], the ideal, steady-state view time for the current epoch.
// For numerical stability, we avoid repetitive conversions between seconds and time.Duration.
// Instead, internally within the controller, we work with float64 in units of seconds.
func (epoch *epochTiming) targetViewTime() float64 {
	return float64(epoch.targetDuration) / float64(epoch.finalView-epoch.firstView+1)
}

// isFollowedBy determines whether nextEpoch is indeed the direct successor of the receiver,
// based on the view ranges of both epochs.
func (et *epochTiming) isFollowedBy(nextEpoch *epochTiming) bool {
	return et.finalView+1 == nextEpoch.firstView
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
	// protocol.Consumer consumes protocol state events
	protocol.Consumer

	config *Config

	state   protocol.State
	log     zerolog.Logger
	metrics module.CruiseCtlMetrics

	// currentEpochTiming holds the timing information for the current epoch (and next epoch if it is committed)
	currentEpochTiming epochTiming
	// nextEpochTiming holds the timing information for the next epoch if it is committed
	nextEpochTiming *epochTiming

	// incorporatedBlocks queues OnBlockIncorporated notifications for subsequent processing by an internal worker routine.
	incorporatedBlocks chan TimedBlock
	// Channel capacity is small and if `incorporatedBlocks` is full we discard new blocks, because the timing targets
	// from the controller only make sense, if the node is not overloaded and swiftly processing new blocks.
	// epochEvents queues functors for processing epoch-related protocol events.
	// Events will be processed in the order they are received (fifo).
	epochEvents     chan func() error
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
		epochEvents:          make(chan func() error, 20),
		proportionalErr:      proportionalErr,
		integralErr:          integralErr,
		latestProposalTiming: atomic.NewPointer[ProposalTiming](nil), // set in initProposalTiming
	}
	ctl.Component = component.NewComponentManagerBuilder().
		AddWorker(ctl.processEventsWorkerLogic).
		Build()

	// initialize state
	err = ctl.initEpochTiming()
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

// initEpochTiming initializes the epochInfo state upon component startup.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) initEpochTiming() error {
	finalSnapshot := ctl.state.Final()

	currentEpochTiming, err := newEpochTiming(finalSnapshot.Epochs().Current())
	if err != nil {
		return fmt.Errorf("failed to retrieve the current epoch's timing information: %w", err)
	}
	ctl.currentEpochTiming = *currentEpochTiming

	phase, err := finalSnapshot.EpochPhase()
	if err != nil {
		return fmt.Errorf("could not check snapshot phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		ctl.nextEpochTiming, err = newEpochTiming(finalSnapshot.Epochs().Next())
		if err != nil {
			return fmt.Errorf("failed to retrieve the next epoch's timing information: %w", err)
		}
		if !currentEpochTiming.isFollowedBy(ctl.nextEpochTiming) {
			return fmt.Errorf("failed to retrieve the next epoch's timing information: %w", err)
		}
	}

	return nil
}

// initProposalTiming initializes the ProposalTiming value upon startup.
// CAUTION: Must be called after initEpochTiming.
func (ctl *BlockTimeController) initProposalTiming(curView uint64) {
	// When disabled, or in epoch fallback, use fallback timing (constant ProposalDuration)
	if !ctl.config.Enabled.Load() {
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

// getProposalTiming returns the controller's latest ProposalTiming. Concurrency safe.
func (ctl *BlockTimeController) getProposalTiming() ProposalTiming {
	pt := ctl.latestProposalTiming.Load()
	if pt == nil { // should never happen, as we always store non-nil instances of ProposalTiming. Though, this extra check makes `GetProposalTiming` universal.
		return nil
	}
	return *pt
}

// TargetPublicationTime is intended to be called by the EventHandler, whenever it
// wants to publish a new proposal. The event handler inputs
//   - proposalView: the view it is proposing for,
//   - timeViewEntered: the time when the EventHandler entered this view
//   - parentBlockId: the ID of the parent block, which the EventHandler is building on
//
// TargetPublicationTime returns the time stamp when the new proposal should be broadcasted.
// For a given view where we are the primary, suppose the actual time we are done building our proposal is P:
//   - if P < TargetPublicationTime(..), then the EventHandler should wait until
//     `TargetPublicationTime` to broadcast the proposal
//   - if P >= TargetPublicationTime(..), then the EventHandler should immediately broadcast the proposal
//
// Note: Technically, our metrics capture the publication delay relative to this function's _latest_ call.
// Currently, the EventHandler is the only caller of this function, and only calls it once per proposal.
//
// Concurrency safe.
func (ctl *BlockTimeController) TargetPublicationTime(proposalView uint64, timeViewEntered time.Time, parentBlockId flow.Identifier) time.Time {
	targetPublicationTime := ctl.getProposalTiming().TargetPublicationTime(proposalView, timeViewEntered, parentBlockId)

	publicationDelay := time.Until(targetPublicationTime)
	// targetPublicationTime should already account for the controller's upper limit of authority (longest view time
	// the controller is allowed to select). However, targetPublicationTime is allowed to be in the past, if the
	// controller want to signal that the proposal should be published asap. We could hypothetically update a past
	// targetPublicationTime to 'now' at every level in the code. However, this time stamp would move into the past
	// immediately, and we would have to update the targetPublicationTime over and over. Instead, we just allow values
	// in the past, thereby making repeated corrections unnecessary. In this model, the code _interpreting_ the value
	// needs to apply the convention a negative publicationDelay essentially means "no delay".
	if publicationDelay < 0 {
		publicationDelay = 0 // Controller can only delay publication of proposal. Hence, the delay is lower-bounded by zero.
	}
	ctl.metrics.ProposalPublicationDelay(publicationDelay)

	return targetPublicationTime
}

// processEventsWorkerLogic is the logic for processing events received from other components.
// This method should be executed by a dedicated worker routine (not concurrency safe).
func (ctl *BlockTimeController) processEventsWorkerLogic(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	done := ctx.Done()
	for {
		// Priority 1: epoch related protocol events.
		select {
		case processEvtFn := <-ctl.epochEvents:
			err := processEvtFn()
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling epoch related event")
				ctx.Throw(err)
			}
		default:
		}

		// Priority 2: OnBlockIncorporated
		select {
		case <-done:
			return
		case block := <-ctl.incorporatedBlocks:
			err := ctl.processIncorporatedBlock(block)
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling OnBlockIncorporated data")
				ctx.Throw(err)
				return
			}
		case processEvtFn := <-ctl.epochEvents:
			err := processEvtFn()
			if err != nil {
				ctl.log.Err(err).Msgf("fatal error handling epoch related event")
				ctx.Throw(err)
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
	latest := ctl.getProposalTiming()
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
	if view <= ctl.currentEpochTiming.finalView { // prevalent case: we are still within the current epoch
		return nil
	}

	// sanity checks, since we are beyond the final view of the most recently processed epoch:
	if ctl.nextEpochTiming == nil { // next epoch timing not initialized
		return fmt.Errorf("sanity check failed: cannot transition without next epoch timing initialized")
	}
	if !ctl.currentEpochTiming.isFollowedBy(ctl.nextEpochTiming) { // non consecutive epochs
		return fmt.Errorf("sanity check failed: invalid epoch transition current epoch (final view: %d) is not followed by next epoch (first view: %d)",
			ctl.currentEpochTiming.finalView, ctl.nextEpochTiming.firstView)
	}
	if view > ctl.nextEpochTiming.finalView { // the block's view should be within the upcoming epoch
		return fmt.Errorf("sanity check failed: curView %d is beyond both current epoch (final view %d) and next epoch (final view %d)",
			view, ctl.currentEpochTiming.finalView, ctl.nextEpochTiming.finalView)
	}

	ctl.currentEpochTiming.firstView = ctl.nextEpochTiming.firstView
	ctl.currentEpochTiming.finalView = ctl.nextEpochTiming.finalView
	ctl.currentEpochTiming.targetDuration = ctl.nextEpochTiming.targetDuration
	ctl.currentEpochTiming.targetEndTime = ctl.nextEpochTiming.targetEndTime
	ctl.nextEpochTiming = nil

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

	previousProposalTiming := ctl.getProposalTiming()
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
	tau := ctl.currentEpochTiming.targetViewTime()                                            // τ: idealized target view time in units of seconds
	viewDurationsRemaining := ctl.currentEpochTiming.finalView + 1 - view                     // k[v]: views remaining in current epoch
	durationRemaining := unix2time(ctl.currentEpochTiming.targetEndTime).Sub(tb.TimeObserved) // Γ[v] = T[v] - t[v], with t[v] ≡ tb.TimeObserved the time when observing the block that triggered the view change

	// Compute instantaneous error term: e[v] = k[v]·τ - T[v] i.e. the projected difference from target switchover
	// and update PID controller's error terms. All UNITS in SECOND.
	instErr := float64(viewDurationsRemaining)*tau - durationRemaining.Seconds()
	propErr := ctl.proportionalErr.AddObservation(instErr)
	itgErr := ctl.integralErr.AddObservation(instErr)
	drivErr := propErr - previousPropErr

	// controller output u[v] in units of second
	u := propErr*ctl.config.KP + itgErr*ctl.config.KI + drivErr*ctl.config.KD

	// compute the controller output for this observation
	unconstrainedBlockTime := sec2dur(tau - u) // desired time between parent and child block, in units of seconds
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
		Dur("controller_output", sec2dur(u)).
		Dur("unconstrained_block_time", unconstrainedBlockTime).
		Dur("constrained_block_time", constrainedBlockTime).
		Msg("measured error upon view change")

	ctl.metrics.PIDError(propErr, itgErr, drivErr)
	ctl.metrics.ControllerOutput(sec2dur(u))
	ctl.metrics.TargetProposalDuration(proposalTiming.ConstrainedBlockTime())

	ctl.storeProposalTiming(proposalTiming)
	return nil
}

// processEpochCommittedPhaseStarted processes the EpochExtended notification, which the Protocol
// State emits when we finalize the first block whose Protocol State further extends the current
// epoch. The next epoch should not be committed so far, because epoch extension are only added
// when there is no subsequent epoch that we could transition into but the current epoch is nearing
// its end. Specifically, we memorize the updated timing information in the BlockTimeController.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processEpochExtended(first *flow.Header) error {
	currEpochWithExtension, err := newEpochTiming(ctl.state.AtHeight(first.Height).Epochs().Current())
	if err != nil {
		return fmt.Errorf("failed to get new epoch timing: %w", err)
	}

	// sanity check: ensure the final view of the current epoch monotonically increases
	if currEpochWithExtension.finalView < ctl.currentEpochTiming.finalView {
		return fmt.Errorf("final view of epoch must be monotonically increases, but is decreasing from %d to %d", ctl.currentEpochTiming.finalView, currEpochWithExtension.finalView)
	}

	if currEpochWithExtension.finalView == ctl.currentEpochTiming.finalView {
		return nil
	}

	ctl.currentEpochTiming.finalView = currEpochWithExtension.finalView
	ctl.currentEpochTiming.targetEndTime = currEpochWithExtension.targetEndTime
	ctl.currentEpochTiming.targetDuration = currEpochWithExtension.targetDuration

	return nil
}

// processEpochCommittedPhaseStarted processes the EpochCommittedPhaseStarted notification, which
// the consensus component emits when we finalize the first block of the Epoch Committed phase.
// Specifically, we memorize the next epoch's timing information in the BlockTimeController.
// No errors are expected during normal operation.
func (ctl *BlockTimeController) processEpochCommittedPhaseStarted(first *flow.Header) error {
	var err error
	snapshot := ctl.state.AtHeight(first.Height)
	ctl.nextEpochTiming, err = newEpochTiming(snapshot.Epochs().Next())
	if err != nil {
		return fmt.Errorf("failed to retrieve the next epoch's timing information: %w", err)
	}
	if !ctl.currentEpochTiming.isFollowedBy(ctl.nextEpochTiming) {
		return fmt.Errorf("failed to retrieve the next epoch's timing information: %w", err)
	}
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

// EpochExtended listens to `EpochExtended` protocol notifications. The notification is queued
// for async processing by the worker. We must process _all_ `EpochExtended` notifications.
func (ctl *BlockTimeController) EpochExtended(_ uint64, first *flow.Header, _ flow.EpochExtension) {
	ctl.epochEvents <- func() error {
		return ctl.processEpochExtended(first)
	}
}

// EpochCommittedPhaseStarted ingests the respective protocol notifications. The notification is
// queued for async processing by the worker. We must process _all_ `EpochExtended` notifications.
func (ctl *BlockTimeController) EpochCommittedPhaseStarted(_ uint64, first *flow.Header) {
	ctl.epochEvents <- func() error {
		return ctl.processEpochCommittedPhaseStarted(first)
	}
}

// time2unix converts a time.Time to UNIX time represented as a uint64.
// Returned timestamp is precise to within one second of input.
func time2unix(t time.Time) uint64 {
	return uint64(t.Unix())
}

// unix2time converts a UNIX timestamp represented as a uint64 to a time.Time.
func unix2time(unix uint64) time.Time {
	return time.Unix(int64(unix), 0)
}

// sec2dur converts a floating-point number of seconds to a time.Duration.
func sec2dur(sec float64) time.Duration {
	return time.Duration(int64(sec * float64(time.Second)))
}
