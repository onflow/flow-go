package committees

import (
	"fmt"
	"sync"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/prg"
)

// staticEpochInfo contains leader selection and the initial committee for one epoch.
// This data structure must not be mutated after construction.
type staticEpochInfo struct {
	firstView            uint64                  // first view of the epoch (inclusive)
	finalView            uint64                  // final view of the epoch (inclusive)
	randomSource         []byte                  // random source of epoch
	leaders              *leader.LeaderSelection // pre-computed leader selection for the epoch
	initialCommittee     flow.IdentitySkeletonList
	initialCommitteeMap  map[flow.Identifier]*flow.IdentitySkeleton
	weightThresholdForQC uint64 // computed based on initial committee weights
	weightThresholdForTO uint64 // computed based on initial committee weights
	dkg                  hotstuff.DKG
}

// newStaticEpochInfo returns the static epoch information from the epoch.
// This can be cached and used for all by-view queries for this epoch.
func newStaticEpochInfo(epoch protocol.Epoch) (*staticEpochInfo, error) {
	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not get first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not get final view: %w", err)
	}
	randomSource, err := epoch.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get random source: %w", err)
	}
	leaders, err := leader.SelectionForConsensus(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not get leader selection: %w", err)
	}
	initialIdentities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not initial identities: %w", err)
	}
	initialCommittee := initialIdentities.Filter(filter.IsConsensusCommitteeMember).ToSkeleton()
	dkg, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("could not get dkg: %w", err)
	}

	totalWeight := initialCommittee.TotalWeight()
	epochInfo := &staticEpochInfo{
		firstView:            firstView,
		finalView:            finalView,
		randomSource:         randomSource,
		leaders:              leaders,
		initialCommittee:     initialCommittee,
		initialCommitteeMap:  initialCommittee.Lookup(),
		weightThresholdForQC: WeightThresholdToBuildQC(totalWeight),
		weightThresholdForTO: WeightThresholdToTimeout(totalWeight),
		dkg:                  dkg,
	}
	return epochInfo, nil
}

// newEmergencyFallbackEpoch creates an artificial fallback epoch generated from
// the last committed epoch at the time epoch emergency fallback is triggered.
// The fallback epoch:
// * begins after the last committed epoch
// * lasts until the next spork (estimated 6 months)
// * has the same static committee as the last committed epoch
func newEmergencyFallbackEpoch(lastCommittedEpoch *staticEpochInfo) (*staticEpochInfo, error) {
	rng, err := prg.New(lastCommittedEpoch.randomSource, prg.ConsensusLeaderSelection, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create rng from seed: %w", err)
	}
	leaders, err := leader.ComputeLeaderSelection(
		lastCommittedEpoch.finalView+1,
		rng,
		leader.EstimatedSixMonthOfViews,
		lastCommittedEpoch.initialCommittee,
	)
	if err != nil {
		return nil, fmt.Errorf("could not compute leader selection for fallback epoch: %w", err)
	}
	epochInfo := &staticEpochInfo{
		firstView:            lastCommittedEpoch.finalView + 1,
		finalView:            lastCommittedEpoch.finalView + leader.EstimatedSixMonthOfViews,
		randomSource:         lastCommittedEpoch.randomSource,
		leaders:              leaders,
		initialCommittee:     lastCommittedEpoch.initialCommittee,
		initialCommitteeMap:  lastCommittedEpoch.initialCommitteeMap,
		weightThresholdForQC: lastCommittedEpoch.weightThresholdForQC,
		weightThresholdForTO: lastCommittedEpoch.weightThresholdForTO,
		dkg:                  lastCommittedEpoch.dkg,
	}
	return epochInfo, nil
}

// Consensus represents the main committee for consensus nodes. The consensus
// committee might be active for multiple successive epochs.
type Consensus struct {
	state                  protocol.State              // the protocol state
	me                     flow.Identifier             // the node ID of this node
	mu                     sync.RWMutex                // protects access to epochs
	epochs                 map[uint64]*staticEpochInfo // cache of initial committee & leader selection per epoch
	committedEpochsCh      chan *flow.Header           // protocol events for newly committed epochs (the first block of the epoch is passed over the channel)
	epochEmergencyFallback chan struct{}               // protocol event for epoch emergency fallback
	isEpochFallbackHandled *atomic.Bool                // ensure we only inject fallback epoch once
	events.Noop                                        // implements protocol.Consumer
	component.Component
}

var _ protocol.Consumer = (*Consensus)(nil)
var _ hotstuff.Replicas = (*Consensus)(nil)
var _ hotstuff.DynamicCommittee = (*Consensus)(nil)

func NewConsensusCommittee(state protocol.State, me flow.Identifier) (*Consensus, error) {
	com := &Consensus{
		state:                  state,
		me:                     me,
		epochs:                 make(map[uint64]*staticEpochInfo),
		committedEpochsCh:      make(chan *flow.Header, 1),
		epochEmergencyFallback: make(chan struct{}, 1),
		isEpochFallbackHandled: atomic.NewBool(false),
	}

	com.Component = component.NewComponentManagerBuilder().
		AddWorker(com.handleProtocolEvents).
		Build()

	final := state.Final()

	// pre-compute leader selection for all presently relevant committed epochs
	epochs := make([]protocol.Epoch, 0, 3)
	// we always prepare the current epoch
	epochs = append(epochs, final.Epochs().Current())

	// we prepare the previous epoch, if one exists
	exists, err := protocol.PreviousEpochExists(final)
	if err != nil {
		return nil, fmt.Errorf("could not check previous epoch exists: %w", err)
	}
	if exists {
		epochs = append(epochs, final.Epochs().Previous())
	}

	// we prepare the next epoch, if it is committed
	phase, err := final.Phase()
	if err != nil {
		return nil, fmt.Errorf("could not check epoch phase: %w", err)
	}
	if phase == flow.EpochPhaseCommitted {
		epochs = append(epochs, final.Epochs().Next())
	}

	for _, epoch := range epochs {
		_, err = com.prepareEpoch(epoch)
		if err != nil {
			return nil, fmt.Errorf("could not prepare initial epochs: %w", err)
		}
	}

	// if epoch emergency fallback was triggered, inject the fallback epoch
	triggered, err := state.Params().EpochFallbackTriggered()
	if err != nil {
		return nil, fmt.Errorf("could not check epoch fallback: %w", err)
	}
	if triggered {
		err = com.onEpochEmergencyFallbackTriggered()
		if err != nil {
			return nil, fmt.Errorf("could not prepare emergency fallback epoch: %w", err)
		}
	}

	return com, nil
}

// IdentitiesByBlock returns the identities of all authorized consensus participants at the given block.
// The order of the identities is the canonical order.
// ERROR conditions:
//   - state.ErrUnknownSnapshotReference if the blockID is for an unknown block
func (c *Consensus) IdentitiesByBlock(blockID flow.Identifier) (flow.IdentityList, error) {
	il, err := c.state.AtBlockID(blockID).Identities(filter.IsVotingConsensusCommitteeMember)
	if err != nil {
		return nil, fmt.Errorf("could not identities at block %x: %w", blockID, err) // state.ErrUnknownSnapshotReference or exception
	}
	return il, nil
}

// IdentityByBlock returns the identity of the node with the given node ID at the given block.
// ERROR conditions:
//   - model.InvalidSignerError if participantID does NOT correspond to an authorized HotStuff participant at the specified block.
//   - state.ErrUnknownSnapshotReference if the blockID is for an unknown block
func (c *Consensus) IdentityByBlock(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	identity, err := c.state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil, model.NewInvalidSignerErrorf("id %v is not a valid node id: %w", nodeID, err)
		}
		return nil, fmt.Errorf("could not get identity for node ID %x: %w", nodeID, err) // state.ErrUnknownSnapshotReference or exception
	}
	if !filter.IsVotingConsensusCommitteeMember(identity) {
		return nil, model.NewInvalidSignerErrorf("node %v is not an authorized hotstuff voting participant", nodeID)
	}
	return identity, nil
}

// IdentitiesByEpoch returns the committee identities in the epoch which contains
// the given view.
// CAUTION: This method considers epochs outside of Previous, Current, Next, w.r.t. the
// finalized block, to be unknown. https://github.com/onflow/flow-go/issues/4085
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) IdentitiesByEpoch(view uint64) (flow.IdentitySkeletonList, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	return epochInfo.initialCommittee, nil
}

// IdentityByEpoch returns the identity for the given node ID, in the epoch which
// contains the given view.
// CAUTION: This method considers epochs outside of Previous, Current, Next, w.r.t. the
// finalized block, to be unknown. https://github.com/onflow/flow-go/issues/4085
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   - model.InvalidSignerError if nodeID was not listed by the Epoch Setup event as an
//     authorized consensus participants.
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.IdentitySkeleton, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	identity, ok := epochInfo.initialCommitteeMap[participantID]
	if !ok {
		return nil, model.NewInvalidSignerErrorf("id %v is not a valid node id", participantID)
	}
	return identity, nil
}

// LeaderForView returns the node ID of the leader for the given view.
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return flow.ZeroID, err
	}
	leaderID, err := epochInfo.leaders.LeaderForView(view)
	if leader.IsInvalidViewError(err) {
		// an invalid view error indicates that no leader was computed for this view
		// this is a fatal internal error, because the view necessarily is within an
		// epoch for which we have pre-computed leader selection
		return flow.ZeroID, fmt.Errorf("unexpected inconsistency in epoch view spans for view %d: %v", view, err)
	}
	if err != nil {
		return flow.ZeroID, err
	}
	return leaderID, nil
}

// QuorumThresholdForView returns the minimum weight required to build a valid
// QC in the given view. The weight threshold only changes at epoch boundaries
// and is computed based on the initial committee weights.
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) QuorumThresholdForView(view uint64) (uint64, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return 0, err
	}
	return epochInfo.weightThresholdForQC, nil
}

func (c *Consensus) Self() flow.Identifier {
	return c.me
}

// TimeoutThresholdForView returns the minimum weight of observed timeout objects
// to safely immediately timeout for the current view. The weight threshold only
// changes at epoch boundaries and is computed based on the initial committee weights.
func (c *Consensus) TimeoutThresholdForView(view uint64) (uint64, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return 0, err
	}
	return epochInfo.weightThresholdForTO, nil
}

// DKG returns the DKG for epoch which includes the given view.
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) DKG(view uint64) (hotstuff.DKG, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	return epochInfo.dkg, nil
}

// handleProtocolEvents processes queued Epoch events `EpochCommittedPhaseStarted`
// and `EpochEmergencyFallbackTriggered`. This function permanently utilizes a worker
// routine until the `Component` terminates.
// When we observe a new epoch being committed, we compute
// the leader selection and cache static info for the epoch. When we observe
// epoch emergency fallback being triggered, we inject a fallback epoch.
func (c *Consensus) handleProtocolEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case block := <-c.committedEpochsCh:
			epoch := c.state.AtBlockID(block.ID()).Epochs().Next()
			_, err := c.prepareEpoch(epoch)
			if err != nil {
				ctx.Throw(err)
			}
		case <-c.epochEmergencyFallback:
			err := c.onEpochEmergencyFallbackTriggered()
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// EpochCommittedPhaseStarted informs the `committee.Consensus` that the block starting the Epoch Committed Phase has been finalized.
func (c *Consensus) EpochCommittedPhaseStarted(_ uint64, first *flow.Header) {
	c.committedEpochsCh <- first
}

// EpochEmergencyFallbackTriggered passes the protocol event to the worker thread.
func (c *Consensus) EpochEmergencyFallbackTriggered() {
	c.epochEmergencyFallback <- struct{}{}
}

// onEpochEmergencyFallbackTriggered handles the protocol event for emergency epoch
// fallback mode being triggered. When this occurs, we inject a fallback epoch
// to the committee which extends the current epoch.
// This method must also be called on initialization, if emergency fallback mode
// was triggered in the past.
// No errors are expected during normal operation.
func (c *Consensus) onEpochEmergencyFallbackTriggered() error {

	// we respond to epoch fallback being triggered at most once, therefore
	// the core logic is protected by an atomic bool.
	// although it is only valid for epoch fallback to be triggered once per spork,
	// we must account for repeated delivery of protocol events.
	if !c.isEpochFallbackHandled.CompareAndSwap(false, true) {
		return nil
	}

	currentEpochCounter, err := c.state.Final().Epochs().Current().Counter()
	if err != nil {
		return fmt.Errorf("could not get current epoch counter: %w", err)
	}

	c.mu.RLock()
	// sanity check: current epoch must be cached already
	currentEpoch, ok := c.epochs[currentEpochCounter]
	if !ok {
		c.mu.RUnlock()
		return fmt.Errorf("epoch fallback: could not find current epoch (counter=%d) info", currentEpochCounter)
	}
	// sanity check: next epoch must never be committed, therefore must not be cached
	_, ok = c.epochs[currentEpochCounter+1]
	c.mu.RUnlock()
	if ok {
		return fmt.Errorf("epoch fallback: next epoch (counter=%d) is cached contrary to expectation", currentEpochCounter+1)
	}

	fallbackEpoch, err := newEmergencyFallbackEpoch(currentEpoch)
	if err != nil {
		return fmt.Errorf("could not construct fallback epoch: %w", err)
	}

	// cache the epoch info
	c.mu.Lock()
	c.epochs[currentEpochCounter+1] = fallbackEpoch
	c.mu.Unlock()

	return nil
}

// staticEpochInfoByView retrieves the previously cached static epoch info for
// the epoch which includes the given view. If no epoch is known for the given
// view, we will attempt to cache the next epoch.
//
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) staticEpochInfoByView(view uint64) (*staticEpochInfo, error) {

	// look for an epoch matching this view for which we have already pre-computed
	// leader selection. Epochs last ~500k views, so we find the epoch here 99.99%
	// of the time. Since epochs are long-lived and we only cache the most recent 3,
	// this linear map iteration is inexpensive.
	c.mu.RLock()
	for _, epoch := range c.epochs {
		if epoch.firstView <= view && view <= epoch.finalView {
			c.mu.RUnlock()
			return epoch, nil
		}
	}
	c.mu.RUnlock()

	return nil, model.ErrViewForUnknownEpoch
}

// prepareEpoch pre-computes and stores the static epoch information for the
// given epoch, including leader selection. Calling prepareEpoch multiple times
// for the same epoch returns cached static epoch information.
// Input must be a committed epoch.
// No errors are expected during normal operation.
func (c *Consensus) prepareEpoch(epoch protocol.Epoch) (*staticEpochInfo, error) {

	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get counter for epoch to prepare: %w", err)
	}

	// this is a no-op if we have already computed static info for this epoch
	c.mu.RLock()
	epochInfo, exists := c.epochs[counter]
	c.mu.RUnlock()
	if exists {
		return epochInfo, nil
	}

	epochInfo, err = newStaticEpochInfo(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not create static epoch info for epch %d: %w", counter, err)
	}

	// sanity check: ensure new epoch has contiguous views with the prior epoch
	c.mu.RLock()
	prevEpochInfo, exists := c.epochs[counter-1]
	c.mu.RUnlock()
	if exists {
		if epochInfo.firstView != prevEpochInfo.finalView+1 {
			return nil, fmt.Errorf("non-contiguous view ranges between consecutive epochs (epoch_%d=[%d,%d], epoch_%d=[%d,%d])",
				counter-1, prevEpochInfo.firstView, prevEpochInfo.finalView,
				counter, epochInfo.firstView, epochInfo.finalView)
		}
	}

	// cache the epoch info
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epochs[counter] = epochInfo
	// now prune any old epochs, if we have exceeded our maximum of 3
	// if we have fewer than 3 epochs, this is a no-op
	c.pruneEpochInfo()
	return epochInfo, nil
}

// pruneEpochInfo removes any epochs older than the most recent 3.
// NOTE: Not safe for concurrent use - the caller must first acquire the lock.
func (c *Consensus) pruneEpochInfo() {
	// find the maximum counter, including the epoch we just computed
	max := uint64(0)
	for counter := range c.epochs {
		if counter > max {
			max = counter
		}
	}

	// remove any epochs which aren't within the most recent 3
	for counter := range c.epochs {
		if counter+3 <= max {
			delete(c.epochs, counter)
		}
	}
}
