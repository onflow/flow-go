package committees

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// epochLeaders is a wrapper structure containing the initial consensus committee
// and the raw leader selection for an epoch.
type epochLeaders struct {
	// pre-computed leader selection for this epoch
	selection *leader.LeaderSelection
	// initial set of consensus committee members for this epoch, used only for
	// mapping a leader index to node ID
	// CAUTION: does not contain up-to-date weight/ejection info
	identities flow.IdentityList
}

// a filter that returns all members of the consensus committee allowed to vote
var consensusMemberFilter = filter.And(
	filter.HasStake(true),              // must have non-zero weight
	filter.HasRole(flow.RoleConsensus), // must be a consensus node
	filter.Not(filter.Ejected),         // must not be ejected
)

var leadersForEpochNotYetComputedErr = fmt.Errorf("leader selection for epoch not yet computed")

// Consensus represents the main committee for consensus nodes. The consensus
// committee persists across epochs.
type Consensus struct {
	sync.RWMutex
	state protocol.ReadOnlyState // the protocol state
	me    flow.Identifier        // the node ID of this node
	// TODO use uint16 in leader selection impl to halve memory usage
	// TODO delete old entries, this uses ~200kb memory/day with above optimization
	leaders map[uint64]*epochLeaders // pre-computed leader selection for each epoch
}

func NewConsensusCommittee(state protocol.ReadOnlyState, me flow.Identifier) (*Consensus, error) {

	com := &Consensus{
		state:   state,
		me:      me,
		leaders: make(map[uint64]*epochLeaders),
	}

	epochs := state.Final().Epochs()

	// pre-compute leader selection for current epoch
	current := epochs.Current()
	err := com.prepareLeaderSelection(current)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for current epoch: %w", err)
	}

	// Pre-compute leader selection for previous epoch, if it exists.
	//
	// This ensures we always know about leader selection for at least one full
	// epoch into the past, ensuring we are able to not only determine the leader
	// for block proposals we receive, but also adjudicate consensus-related
	// challenges up to one epoch into the past.
	previous := epochs.Previous()
	_, err = previous.Counter()
	// if there is no previous epoch, return the committee as-is
	if errors.Is(err, protocol.ErrNoPreviousEpoch) {
		return com, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not get previous epoch: %w", err)
	}

	err = com.prepareLeaderSelection(previous)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for previous epoch: %w", err)
	}

	return com, nil
}

func (c *Consensus) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	return c.state.AtBlockID(blockID).Identities(filter.And(
		consensusMemberFilter,
		selector,
	))
}

func (c *Consensus) Identity(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	identity, err := c.state.AtBlockID(blockID).Identity(nodeID)
	if err != nil {
		return nil, fmt.Errorf("could not get identity for node ID %x: %w", nodeID, err)
	}
	if !consensusMemberFilter(identity) {
		return nil, fmt.Errorf("node with ID %x is not a valid consensus committee member at block %x", nodeID, blockID)
	}
	return identity, nil
}

func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	// try to retrieve the leader from a pre-computed LeaderSelection
	id, err := c.precomputedLeaderForView(view)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, leadersForEpochNotYetComputedErr) {
		return flow.ZeroID, err
	}
	// we only reach the following code, if we got a leadersForEpochNotYetComputedErr

	// STEP 2 - we haven't yet computed leader selection for an epoch containing
	// the requested view. We compute leader selection for the current and previous
	// epoch (w.r.t. the finalized head) at initialization then compute leader
	// selection for the next epoch when we encounter any view for which we don't
	// know the leader. The series of epochs we have computed leaders for is
	// strictly consecutive, meaning we know the leader for all views V where:
	//
	//   V >= oldestEpoch.firstView && V <= newestEpoch.finalView
	//
	// Thus, the requested view is either before oldestEpoch.firstView or after
	// newestEpoch.finalView.
	//
	// CASE 1: V < oldestEpoch.firstView
	// If the view is before the first view we've computed the leader for, this
	// represents an invalid query because we only guarantee the protocol state
	// will contain epoch information for the current, previous, and next epoch -
	// such a query must be for a view within an epoch at least TWO epochs before
	// the current epoch when we started up. This is considered an invalid query.
	//
	// CASE 2: V > newestEpoch.finalView
	// If the view is after the last view we've computed the leader for, we
	// assume the view is within the next epoch (w.r.t. the finalized head).
	// This assumption is equivalent to assuming that we build at least one
	// block in every epoch, which is anyway a requirement for valid epochs.
	//
	c.Lock()
	defer c.Unlock()
	next := c.state.Final().Epochs().Next()
	err = c.prepareLeaderSelection(next)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not compute leader selection for next epoch: %w", err)
	}
	nextCounter, err := next.Counter()
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get next epoch counter: %w", err)
	}

	// if we get to this point, we are guaranteed to have inserted the leader
	// selection for the next epoch
	nextEpochLeaders := c.leaders[nextCounter]
	index, err := nextEpochLeaders.selection.LeaderIndexForView(view)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get leader index after computing next epoch (%d): %w", nextCounter, err)
	}
	return nextEpochLeaders.identities[index].NodeID, nil
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
//   * leadersForEpochNotYetComputedErr [sentinel error] if there is no Epoch for view stored in `c.leaders`
//   * unspecific error in case of unexpected problems and bugs
func (c *Consensus) precomputedLeaderForView(view uint64) (flow.Identifier, error) {
	c.RLock()
	defer c.RUnlock()
	// STEP 1 - look for an epoch matching this view for which we have already
	// pre-computed leader selection. Epochs last ~500k views, so we find the
	// epoch here 99.99% of the time. Since epochs are long-lived, it is fine
	// for this to be linear in the number of epochs we have observed.
	for _, epoch := range c.leaders {
		if view >= epoch.selection.FirstView() && view <= epoch.selection.FinalView() {
			index, err := epoch.selection.LeaderIndexForView(view)
			if err != nil {
				return flow.ZeroID, fmt.Errorf("could not get leader index for view: %w", err)
			}
			return epoch.identities[index].NodeID, nil
		}
	}
	return flow.ZeroID, leadersForEpochNotYetComputedErr
}

// prepareLeaderSelection pre-computes and stores the leader selection for the
// given epoch. Computing leader selection for the same epoch multiple times
// is a no-op.
func (c *Consensus) prepareLeaderSelection(epoch protocol.Epoch) error {

	counter, err := epoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get counter for current epoch: %w", err)
	}
	// this is a no-op if we have already computed leaders for this epoch
	_, exists := c.leaders[counter]
	if exists {
		return nil
	}

	identities, err := epoch.InitialIdentities()
	if err != nil {
		return fmt.Errorf("could not get initial identities for current epoch: %w", err)
	}
	selection, err := leader.SelectionForConsensus(epoch)
	if err != nil {
		return fmt.Errorf("could not get leader selection for current epoch: %w", err)
	}

	c.leaders[counter] = &epochLeaders{
		selection:  selection,
		identities: identities.Filter(consensusMemberFilter),
	}
	return nil
}
