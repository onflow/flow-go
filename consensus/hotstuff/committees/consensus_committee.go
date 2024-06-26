package committees

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// epochInfo contains leader selection and the initial committee for one epoch.
// TODO more docs
type epochInfo struct {
	*leader.LeaderSelection // pre-computed leader selection for the epoch
	initialCommittee        flow.IdentitySkeletonList
	initialCommitteeMap     map[flow.Identifier]*flow.IdentitySkeleton
	weightThresholdForQC    uint64 // computed based on initial committee weights
	weightThresholdForTO    uint64 // computed based on initial committee weights
	dkg                     hotstuff.DKG
}

// newEpochInfo retrieves the committee information and computes leader selection.
// This can be cached and used for all by-view queries for this epoch.
func newEpochInfo(epoch protocol.Epoch) (*epochInfo, error) {
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
	epochInfo := &epochInfo{
		LeaderSelection:      leaders,
		initialCommittee:     initialCommittee,
		initialCommitteeMap:  initialCommittee.Lookup(),
		weightThresholdForQC: WeightThresholdToBuildQC(totalWeight),
		weightThresholdForTO: WeightThresholdToTimeout(totalWeight),
		dkg:                  dkg,
	}
	return epochInfo, nil
}

// eventHandlerFunc represents an event wrapped in a closure which will execute any required local
// state changes upon execution by the single event processing worker goroutine.
// No errors are expected under normal conditions.
type eventHandlerFunc func() error

// Consensus represents the main committee for consensus nodes. The consensus
// committee might be active for multiple successive epochs.
type Consensus struct {
	state  protocol.State        // the protocol state
	me     flow.Identifier       // the node ID of this node
	mu     sync.RWMutex          // protects access to epochs
	epochs map[uint64]*epochInfo // cache of initial committee & leader selection per epoch
	// TODO do we need to strictly order events across types? Yes, I think we do
	committedEpochsCh chan *flow.Header // protocol events for newly committed epochs (the first block of the epoch is passed over the channel)
	events            chan eventHandlerFunc
	events.Noop       // implements protocol.Consumer
	component.Component
}

var _ protocol.Consumer = (*Consensus)(nil)
var _ hotstuff.Replicas = (*Consensus)(nil)
var _ hotstuff.DynamicCommittee = (*Consensus)(nil)

func NewConsensusCommittee(state protocol.State, me flow.Identifier) (*Consensus, error) {
	com := &Consensus{
		state:             state,
		me:                me,
		epochs:            make(map[uint64]*epochInfo),
		committedEpochsCh: make(chan *flow.Header, 1),
		events:            make(chan eventHandlerFunc, 5),
	}

	com.Component = component.NewComponentManagerBuilder().
		AddWorker(com.handleProtocolEvents).
		Build()

	final := state.Final()

	// pre-compute leader selection for all presently relevant committed epochs
	epochs := make([]protocol.Epoch, 0, 3)

	// we prepare the previous epoch, if one exists
	exists, err := protocol.PreviousEpochExists(final)
	if err != nil {
		return nil, fmt.Errorf("could not check previous epoch exists: %w", err)
	}
	if exists {
		epochs = append(epochs, final.Epochs().Previous())
	}

	// we always prepare the current epoch
	epochs = append(epochs, final.Epochs().Current())

	// we prepare the next epoch, if it is committed
	phase, err := final.EpochPhase()
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
	epochInfo, err := c.epochInfoByView(view)
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
	epochInfo, err := c.epochInfoByView(view)
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

	epochInfo, err := c.epochInfoByView(view)
	if err != nil {
		return flow.ZeroID, err
	}
	leaderID, err := epochInfo.LeaderForView(view)
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
	epochInfo, err := c.epochInfoByView(view)
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
	epochInfo, err := c.epochInfoByView(view)
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
	epochInfo, err := c.epochInfoByView(view)
	if err != nil {
		return nil, err
	}
	return epochInfo.dkg, nil
}

// TODO docs
// handleProtocolEvents processes queued Epoch events `EpochCommittedPhaseStarted`
// and `EpochFallbackModeTriggered`. This function permanently utilizes a worker
// routine until the `Component` terminates.
// When we observe a new epoch being committed, we compute the leader selection and cache the epoch.
func (c *Consensus) handleProtocolEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case handleEvent := <-c.events:
			err := handleEvent()
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// EpochCommittedPhaseStarted informs `committees.Consensus` that the first block in flow.EpochPhaseCommitted has been finalized.
// This event consumer function enqueues an event handler function for the single event handler thread to execute.
func (c *Consensus) EpochCommittedPhaseStarted(_ uint64, first *flow.Header) {
	c.events <- func() error {
		return c.handleEpochCommittedPhaseStarted(first)
	}
}

// EpochExtended informs `committees.Consensus` that a block including a new epoch extension has been finalized.
// This event consumer function enqueues an event handler function for the single event handler thread to execute.
func (c *Consensus) EpochExtended(_ uint64, refBlock *flow.Header, _ flow.EpochExtension) {
	c.events <- func() error {
		return c.handleEpochExtended(refBlock)
	}
}

// handleEpochExtended executes all state changes required upon observing an EpochExtended event.
// This function conforms to eventHandlerFunc.
// When an extension is observed, we re-compute leader selection for the current epoch, taking into
// account the most recent extension (included as of refBlock).
// No errors are expected during normal operation.
func (c *Consensus) handleEpochExtended(refBlock *flow.Header) error {
	currentEpoch := c.state.AtBlockID(refBlock.ID()).Epochs().Current()
	counter, err := currentEpoch.Counter()
	if err != nil {
		return fmt.Errorf("could not read current epoch info: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	epochInfo, ok := c.epochs[counter]
	if !ok {
		return fmt.Errorf("sanity check failed: current epoch committee info does not exist")
	}
	epochInfo.LeaderSelection, err = leader.SelectionForConsensus(currentEpoch)
	if err != nil {
		return fmt.Errorf("could not re-compute leader selection for epoch after extension: %w", err)
	}
	return nil
}

// handleEpochCommittedPhaseStarted executes all state changes required upon observing an EpochCommittedPhaseStarted event.
// This function conforms to eventHandlerFunc.
// When the next epoch is committed, we compute leader selection for the epoch and cache it.
// No errors are expected during normal operation.
func (c *Consensus) handleEpochCommittedPhaseStarted(refBlock *flow.Header) error {
	epoch := c.state.AtBlockID(refBlock.ID()).Epochs().Next()
	_, err := c.prepareEpoch(epoch)
	if err != nil {
		return fmt.Errorf("could not cache data for committed next epoch: %w", err)
	}
	return nil
}

// epochInfoByView retrieves the cached epoch info for the epoch which includes the given view.
// Error returns:
//   - model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known
//   - unspecific error in case of unexpected problems and bugs
func (c *Consensus) epochInfoByView(view uint64) (*epochInfo, error) {

	// look for an epoch matching this view for which we have already pre-computed
	// leader selection. Epochs last ~500k views, so we find the epoch here 99.99%
	// of the time. Since epochs are long-lived and we only cache the most recent 3,
	// this linear map iteration is inexpensive.
	c.mu.RLock()
	for _, epoch := range c.epochs {
		if epoch.FirstView() <= view && view <= epoch.FinalView() {
			c.mu.RUnlock()
			return epoch, nil
		}
	}
	c.mu.RUnlock()

	return nil, model.ErrViewForUnknownEpoch
}

// prepareEpoch pre-computes and caches the epoch information for the given epoch, including leader selection.
// Calling prepareEpoch multiple times for the same epoch returns cached epoch information.
// Input must be a committed epoch.
// No errors are expected during normal operation.
func (c *Consensus) prepareEpoch(epoch protocol.Epoch) (*epochInfo, error) {

	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get counter for epoch to prepare: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// this is a no-op if we have already cached this epoch
	epochInfo, exists := c.epochs[counter]
	if exists {
		return epochInfo, nil
	}

	epochInfo, err = newEpochInfo(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not create epoch info for epoch %d: %w", counter, err)
	}

	// sanity check: ensure new epoch has contiguous views with the prior epoch
	prevEpochInfo, exists := c.epochs[counter-1]
	if exists {
		if epochInfo.FirstView() != prevEpochInfo.FinalView()+1 {
			return nil, fmt.Errorf("non-contiguous view ranges between consecutive epochs (epoch_%d=[%d,%d], epoch_%d=[%d,%d])",
				counter-1, prevEpochInfo.FirstView(), prevEpochInfo.FinalView(),
				counter, epochInfo.FirstView(), epochInfo.FinalView())
		}
	}

	// cache the epoch info
	c.epochs[counter] = epochInfo
	// now prune any old epochs; if we have cached fewer than 3 epochs, this is a no-op
	c.pruneEpochInfo()
	return epochInfo, nil
}

// pruneEpochInfo removes any epochs older than the most recent 3.
// CAUTION: Not safe for concurrent use - the caller must first acquire the lock.
func (c *Consensus) pruneEpochInfo() {
	// find the maximum counter, including the epoch we just computed
	maxCounter := uint64(0)
	for counter := range c.epochs {
		if counter > maxCounter {
			maxCounter = counter
		}
	}

	// remove any epochs which aren't within the most recent 3
	for counter := range c.epochs {
		if counter+3 <= maxCounter {
			delete(c.epochs, counter)
		}
	}
}
