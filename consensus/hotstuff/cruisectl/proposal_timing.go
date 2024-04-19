package cruisectl

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

// ProposalTiming encapsulates the output of the BlockTimeController. On the happy path,
// the controller observes a block and generates a specific ProposalTiming in response.
// For the happy path, the ProposalTiming describes when the child proposal should be
// broadcast.
// However, observations other than blocks might also be used to instantiate ProposalTiming
// objects, e.g. controller instantiation, a disabled controller, etc.
// The purpose of ProposalTiming is to convert the controller output to timing information
// that the EventHandler understands. By convention, ProposalTiming should be treated as
// immutable.
type ProposalTiming interface {
	hotstuff.ProposalDurationProvider

	// ObservationView returns the view of the observation that the controller
	// processed and generated this ProposalTiming instance in response.
	ObservationView() uint64

	// ObservationTime returns the time, when the controller received the
	// leading to the generation of this ProposalTiming instance.
	ObservationTime() time.Time
}

/* *************************************** publishImmediately *************************************** */

// publishImmediately implements ProposalTiming: it returns the time when the view
// was entered as the TargetPublicationTime. By convention, publishImmediately should
// be treated as immutable.
type publishImmediately struct {
	observationView uint64
	observationTime time.Time
}

var _ ProposalTiming = (*publishImmediately)(nil)

func newPublishImmediately(observationView uint64, observationTime time.Time) *publishImmediately {
	return &publishImmediately{
		observationView: observationView,
		observationTime: observationTime,
	}
}

func (pt *publishImmediately) TargetPublicationTime(_ uint64, timeViewEntered time.Time, _ flow.Identifier) time.Time {
	return timeViewEntered
}
func (pt *publishImmediately) ObservationView() uint64             { return pt.observationView }
func (pt *publishImmediately) ObservationTime() time.Time          { return pt.observationTime }
func (pt *publishImmediately) ConstrainedBlockTime() time.Duration { return 0 }

/* *************************************** happyPathBlockTime *************************************** */

// happyPathBlockTime implements ProposalTiming for the happy path. Here, `TimedBlock` _latest_ block that the
// controller observed, and the `unconstrainedBlockTime` for the _child_ of this block.
// This function internally holds the _unconstrained_ view duration as computed by the BlockTimeController. Caution,
// no limits of authority have been applied to this value yet. The final controller output satisfying the limits of
// authority is computed by function `ConstrainedBlockTime()`
//
// For a given view where we are the primary, suppose the parent block we are building on top of has been observed
// at time `t := TimedBlock.TimeObserved` and applying the limits of authority yields `d := ConstrainedBlockTime()`
// Then, `TargetPublicationTime(..)` returns `t + d` as the target publication time for the child block.
//
// By convention, happyPathBlockTime should be treated as immutable.
// TODO: any additional logic for assiting the EventHandler in determining the applied delay should be added to the ControllerViewDuration
type happyPathBlockTime struct {
	TimedBlock   // latest block observed by the controller, including the time stamp when the controller received the block [UTC]
	TimingConfig // timing configuration for the controller, for retrieving the controller's limits of authority

	// unconstrainedBlockTime is the delay, relative to `TimedBlock.TimeObserved` when the controller would
	// like the child block to be published. Caution, no limits of authority have been applied to this value yet.
	// The final controller output after applying the limits of authority is returned by function `ConstrainedBlockTime`
	unconstrainedBlockTime time.Duration // desired duration until releasing the child block, measured from `TimedBlock.TimeObserved`

	constrainedBlockTime time.Duration // block time _after_ applying limits of authority to unconstrainedBlockTime
}

var _ ProposalTiming = (*happyPathBlockTime)(nil)

// newHappyPathBlockTime instantiates a new happyPathBlockTime
func newHappyPathBlockTime(timedBlock TimedBlock, unconstrainedBlockTime time.Duration, timingConfig TimingConfig) *happyPathBlockTime {
	return &happyPathBlockTime{
		TimingConfig:           timingConfig,
		TimedBlock:             timedBlock,
		unconstrainedBlockTime: unconstrainedBlockTime,
		constrainedBlockTime:   min(max(unconstrainedBlockTime, timingConfig.MinViewDuration.Load()), timingConfig.MaxViewDuration.Load()),
	}
}

func (pt *happyPathBlockTime) ObservationView() uint64             { return pt.Block.View }
func (pt *happyPathBlockTime) ObservationTime() time.Time          { return pt.TimeObserved }
func (pt *happyPathBlockTime) ConstrainedBlockTime() time.Duration { return pt.constrainedBlockTime }

// TargetPublicationTime operates in two possible modes:
//  1. If `parentBlockId` matches our `TimedBlock`, i.e. the EventHandler is just building the child block, then
//     we return `TimedBlock.TimeObserved + ConstrainedBlockTime` as the target publication time for the child block.
//  2. If `parentBlockId` does _not_ match our `TimedBlock`, the EventHandler should release the block immediately.
//     This heuristic is based on the intuition that Block time is expected to be very long when deviating from the happy path.
func (pt *happyPathBlockTime) TargetPublicationTime(proposalView uint64, timeViewEntered time.Time, parentBlockId flow.Identifier) time.Time {
	if parentBlockId != pt.Block.BlockID {
		return timeViewEntered // broadcast immediately
	}
	return pt.TimeObserved.Add(pt.ConstrainedBlockTime()) // happy path
}

/* *************************************** fallbackTiming for EFM *************************************** */

// fallbackTiming implements ProposalTiming, for the basic fallback:
// function `TargetPublicationTime(..)` always returns `timeViewEntered + defaultProposalDuration`
type fallbackTiming struct {
	observationView         uint64
	observationTime         time.Time
	defaultProposalDuration time.Duration
}

var _ ProposalTiming = (*fallbackTiming)(nil)

func newFallbackTiming(observationView uint64, observationTime time.Time, defaultProposalDuration time.Duration) *fallbackTiming {
	return &fallbackTiming{
		observationView:         observationView,
		observationTime:         observationTime,
		defaultProposalDuration: defaultProposalDuration,
	}
}

func (pt *fallbackTiming) TargetPublicationTime(_ uint64, timeViewEntered time.Time, _ flow.Identifier) time.Time {
	return timeViewEntered.Add(pt.defaultProposalDuration)
}
func (pt *fallbackTiming) ObservationView() uint64    { return pt.observationView }
func (pt *fallbackTiming) ObservationTime() time.Time { return pt.observationTime }

/* *************************************** auxiliary functions *************************************** */

func min(d1, d2 time.Duration) time.Duration {
	if d1 < d2 {
		return d1
	}
	return d2
}

func max(d1, d2 time.Duration) time.Duration {
	if d1 > d2 {
		return d1
	}
	return d2
}
