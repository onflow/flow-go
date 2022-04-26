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

// ErrViewForUnknownEpoch is returned when a by-view query is made with a view
// outside all cached epochs. This can happen when a query is made for a view in the
// next epoch, if that epoch is not committed yet. This can also happen when an
// old epoch is queried (>3 in the past), even if that epoch does exist in storage.
var ErrViewForUnknownEpoch = fmt.Errorf("by-view query for unknown epoch")

// staticEpochInfo contains leader selection and the initial committee for one epoch.
type staticEpochInfo struct {
	firstView        uint64
	finalView        uint64
	leaders          *leader.LeaderSelection
	initialCommittee flow.IdentityList
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
	initialidentities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not initial identities: %w", err)
	}
	initialCommittee := initialidentities.Filter(filter.IsVotingConsensusCommitteeMember)

	epochInfo := &staticEpochInfo{
		firstView:        firstView,
		finalView:        finalView,
		leaders:          leaders,
		initialCommittee: initialCommittee,
	}
	return epochInfo, nil
}

// Consensus represents the main committee for consensus nodes. The consensus
// committee persists across epochs.
type Consensus struct {
	mu     sync.RWMutex
	state  protocol.State              // the protocol state
	me     flow.Identifier             // the node ID of this node
	epochs map[uint64]*staticEpochInfo // cache of initial committee & leader selection per epoch
}

var _ hotstuff.Committee = (*Consensus)(nil)

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

func (c *Consensus) IdentitiesByBlock(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	il, err := c.state.AtBlockID(blockID).Identities(filter.And(
		filter.IsVotingConsensusCommitteeMember,
		selector,
	))
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

func (c *Consensus) IdentitiesByEpoch(view uint64, selector flow.IdentityFilter) (flow.IdentityList, error) {

}

func (c *Consensus) IdentityByEpoch(view uint64, nodeID flow.Identifier) (*flow.Identity, error) {

}

// LeaderForView returns the node ID of the leader for the given view. Returns
// the following errors:
//  * epoch containing the requested view has not been set up (protocol.ErrNextEpochNotSetup)
//  * epoch is too far in the past (leader.InvalidViewError)
//  * any other error indicates an unexpected internal error
//
// TODO: Update protocol state to trigger EECC early using safety threshold
//   * see https://github.com/dapperlabs/flow-go/issues/6227 for details
//   * we no longer need EECC logic here, because the protocol state will
//     inject a "next" fallback epoch carrying over the last committee until
//     the next spork, which we will query here
func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	id, err := c.precomputedLeaderForView(view)
	// happy path - we already pre-computed the leader for this view
	if err == nil {
		return id, nil
	}
	// unexpected error
	if err != nil && !errors.Is(err, ErrViewForUnknownEpoch) {
		return flow.ZeroID, fmt.Errorf("unexpected error retrieving precomputed leader for view: %w", err)
	}

	// at this point, we know that the epoch for the given view is not cached
	// try to retrieve and cache epoch info for the next epoch
	nextEpochInfo, ok, err := c.tryPrepareNextEpoch()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("unexpected error trying to cache next epoch: %w", err)
	}
	// we don't know about the epoch for this view yet, return the sentinel
	if !ok {
		return flow.ZeroID, ErrViewForUnknownEpoch
	}
	// attempt to retrieve the leader for the newly cached epoch
	// if the view is still not found here, we will return ErrViewForUnknownEpoch
	return nextEpochInfo.leaders.LeaderForView(view)
}

func (c *Consensus) Self() flow.Identifier {
	return c.me
}

func (c *Consensus) DKG(blockID flow.Identifier) (hotstuff.DKG, error) {
	return c.state.AtBlockID(blockID).Epochs().Current().DKG()
}

// precomputedLeaderForView retrieves the leader from the precomputed
// LeaderSelection in `c.leaders`
// Error returns:
//   * ErrViewForUnknownEpoch [sentinel error] if there is no Epoch for view stored in `c.leaders`
//   * unspecific error in case of unexpected problems and bugs
func (c *Consensus) precomputedLeaderForView(view uint64) (flow.Identifier, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// look for an epoch matching this view for which we have already pre-computed
	// leader selection. Epochs last ~500k views, so we find the epoch here 99.99%
	// of the time. Since epochs are long-lived and we only cache the most recent 3,
	// this linear map iteration is inexpensive.
	for _, epoch := range c.epochs {
		if epoch.firstView <= view && view <= epoch.finalView {
			return epoch.leaders.LeaderForView(view)
		}
	}

	return flow.ZeroID, ErrViewForUnknownEpoch
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
	committed, err := protocol.EpochIsCommitted(next)
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

	c.mu.Lock()
	defer c.mu.Unlock()

	// this is a no-op if we have already computed leaders for this epoch
	epochInfo, exists := c.epochs[counter]
	if exists {
		return epochInfo, nil
	}

	epochInfo, err = newStaticEpochInfo(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not create static epoch info for epch %d: %w", counter, err)
	}
	// cache the epoch info
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
