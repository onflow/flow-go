package committees

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// staticEpochInfo contains leader selection and the initial committee for one epoch.
// This data structure must not be mutated after construction.
type staticEpochInfo struct {
	firstView uint64                  // first view of the epoch (inclusive)
	finalView uint64                  // final view of the epoch (inclusive)
	leaders   *leader.LeaderSelection // pre-computed leader selection for the epoch
	// TODO: should use identity skeleton https://github.com/dapperlabs/flow-go/issues/6232
	initialCommittee     flow.IdentityList
	weightThresholdForQC uint64 // computed based on initial committee weights
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
	leaders, err := leader.SelectionForConsensus(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not get leader selection: %w", err)
	}
	initialIdentities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not initial identities: %w", err)
	}
	initialCommittee := initialIdentities.Filter(filter.IsVotingConsensusCommitteeMember)
	dkg, err := epoch.DKG()
	if err != nil {
		return nil, fmt.Errorf("could not get dkg: %w", err)
	}

	epochInfo := &staticEpochInfo{
		firstView:            firstView,
		finalView:            finalView,
		leaders:              leaders,
		initialCommittee:     initialCommittee,
		weightThresholdForQC: WeightThresholdToBuildQC(initialCommittee.TotalWeight()),
		dkg:                  dkg,
	}
	return epochInfo, nil
}

// Consensus represents the main committee for consensus nodes. The consensus
// committee might be active for multiple successive epochs.
type Consensus struct {
	mu     sync.RWMutex
	state  protocol.State              // the protocol state
	me     flow.Identifier             // the node ID of this node
	epochs map[uint64]*staticEpochInfo // cache of initial committee & leader selection per epoch
}

var _ hotstuff.Replicas = (*Consensus)(nil)
var _ hotstuff.DynamicCommittee = (*Consensus)(nil)

func NewConsensusCommittee(state protocol.State, me flow.Identifier) (*Consensus, error) {

	com := &Consensus{
		state:  state,
		me:     me,
		epochs: make(map[uint64]*staticEpochInfo),
	}

	final := state.Final()

	// pre-compute leader selection for current epoch
	current := final.Epochs().Current()
	_, err := com.prepareEpoch(current)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for current epoch: %w", err)
	}

	// Pre-compute leader selection for previous epoch, if it exists.
	//
	// This ensures we always know about leader selection for at least one full
	// epoch into the past, ensuring we are able to not only determine the leader
	// for block proposals we receive, but also adjudicate consensus-related
	// challenges up to one epoch into the past.
	previous := final.Epochs().Previous()
	_, err = previous.Counter()
	// if there is no previous epoch, return the committee as-is
	if errors.Is(err, protocol.ErrNoPreviousEpoch) {
		return com, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get previous epoch: %w", err)
	}

	_, err = com.prepareEpoch(previous)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for previous epoch: %w", err)
	}

	return com, nil
}

func (c *Consensus) IdentitiesByBlock(blockID flow.Identifier) (flow.IdentityList, error) {
	il, err := c.state.AtBlockID(blockID).Identities(filter.IsVotingConsensusCommitteeMember)
	return il, err
}

func (c *Consensus) IdentityByBlock(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	identity, err := c.state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil, model.NewInvalidSignerErrorf("id %v is not a valid node id: %w", nodeID, err)
		}
		return nil, fmt.Errorf("could not get identity for node ID %x: %w", nodeID, err)
	}
	if !filter.IsVotingConsensusCommitteeMember(identity) {
		return nil, model.NewInvalidSignerErrorf("node %v is not an authorized hotstuff voting participant", nodeID)
	}
	return identity, nil
}

// IdentitiesByEpoch returns the committee identities in the epoch which contains
// the given view.
//
// Error returns:
//   * model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   * unspecific error in case of unexpected problems and bugs
//
func (c *Consensus) IdentitiesByEpoch(view uint64) (flow.IdentityList, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	return epochInfo.initialCommittee, nil
}

// IdentityByEpoch returns the identity for the given node ID, in the epoch which
// contains the given view.
//
// Error returns:
//   * model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   * model.InvalidSignerError if nodeID was not listed by the Epoch Setup event as an
//     authorized consensus participants.
//   * unspecific error in case of unexpected problems and bugs
//
func (c *Consensus) IdentityByEpoch(view uint64, nodeID flow.Identifier) (*flow.Identity, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	identity, ok := epochInfo.initialCommittee.ByNodeID(nodeID)
	if !ok {
		return nil, model.NewInvalidSignerErrorf("id %v is not a valid node id: %w", nodeID, err)
	}
	return identity, nil
}

// LeaderForView returns the node ID of the leader for the given view.
//
// Error returns:
//   * model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   * unspecific error in case of unexpected problems and bugs
//
// TODO: Update protocol state to trigger EECC early using safety threshold
//   * see https://github.com/dapperlabs/flow-go/issues/6227 for details
//   * the current implementation assumes #6227 is implemented
//   * we no longer need EECC logic here, because the protocol state will
//     inject a "next" fallback epoch carrying over the last committee until
//     the next spork, which we query here
func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return flow.ZeroID, err
	}
	return epochInfo.leaders.LeaderForView(view)
}

// WeightThresholdForView returns the minimum weight required to build a valid
// QC in the given view. The weight threshold only changes at epoch boundaries
// and is computed based on the initial committee weights.
func (c *Consensus) WeightThresholdForView(view uint64) (uint64, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return 0, err
	}
	return epochInfo.weightThresholdForQC, nil
}

func (c *Consensus) Self() flow.Identifier {
	return c.me
}

// DKG returns the DKG for epoch which includes the given view.
//
// Error returns:
//   * model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known.
//     This is an expected error and must be handled.
//   * unspecific error in case of unexpected problems and bugs
func (c *Consensus) DKG(view uint64) (hotstuff.DKG, error) {
	epochInfo, err := c.staticEpochInfoByView(view)
	if err != nil {
		return nil, err
	}
	return epochInfo.dkg, nil
}

// staticEpochInfoByView retrieves the previously cached static epoch info for
// the epoch which includes the given view. If no epoch is known for the given
// view, we will attempt to cache the next epoch.
//
// Error returns:
//   * model.ErrViewForUnknownEpoch if no committed epoch containing the given view is known
//   * unspecific error in case of unexpected problems and bugs
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

	// at this point, we know that the epoch for the given view is not cached
	// try to retrieve and cache epoch info for the next epoch
	nextEpochInfo, ok, err := c.tryPrepareNextEpoch()
	if err != nil {
		return nil, fmt.Errorf("unexpected error trying to cache next epoch: %w", err)
	}
	// we don't know about the epoch for this view yet, return the sentinel
	if !ok {
		return nil, model.ErrViewForUnknownEpoch
	}
	// we successfully cached the next epoch, return the corresponding static info
	// if it contains the given view
	if nextEpochInfo.firstView <= view && view <= nextEpochInfo.finalView {
		return nextEpochInfo, nil
	}
	return nil, model.ErrViewForUnknownEpoch
}

// tryPrepareNextEpoch tries to cache the next epoch, returning the cached static
// epoch info if caching is successful.
//
// Returns:
// * nextEpochInfo, true, nil if the next epoch info was successfully cached
// * nil, false, nil if the next epoch is not committed
// * nil, false, err for any unexpected errors
//
// TODO: trigger this via an event from protocol state instead
//   * previously the asynchrony of protocol events was not acceptable, because any
//     unknown view errors for LeaderForView were fatal
//   * now, we handle these errors, since we may validate messages from future views -
//     so long as we eventually cache a newly committed epoch, liveness is not compromised
func (c *Consensus) tryPrepareNextEpoch() (*staticEpochInfo, bool, error) {
	next := c.state.Final().Epochs().Next()
	committed, err := protocol.IsEpochCommitted(next)
	if err != nil {
		return nil, false, fmt.Errorf("could not check if epoch is committed: %w", err)
	}
	if !committed {
		return nil, false, nil
	}
	epochInfo, err := c.prepareEpoch(next)
	if err != nil {
		return nil, false, fmt.Errorf("could not prepare next epoch: %w", err)
	}
	return epochInfo, true, nil
}

// prepareEpoch pre-computes and stores the static epoch information for the
// given epoch, including leader selection. Calling prepareEpoch multiple times
// for the same epoch returns cached static epoch information.
//
// Input must be a committed epoch.
//
func (c *Consensus) prepareEpoch(epoch protocol.Epoch) (*staticEpochInfo, error) {

	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get counter for current epoch: %w", err)
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

// pruneEpochInfo removes any epochs
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
