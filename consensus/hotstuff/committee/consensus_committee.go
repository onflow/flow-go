package committee

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committee/leader"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

type EpochLeaders struct {
	selection  *LeaderSelection  // pre-computed leader selection for this epoch
	identities flow.IdentityList // initial set of consensus committee members for this epoch
}

var consensusMemberFilter = filter.And(
	filter.HasStake(true),              // must have non-zero weight
	filter.HasRole(flow.RoleConsensus), // must be a consensus node
	filter.Not(filter.Ejected),         // must not be ejected
)

// Consensus represents the main committee for consensus nodes. The consensus
// committee persists across epochs.
type Consensus struct {
	state   protocol.ReadOnlyState   // the protocol state
	me      flow.Identifier          // the node ID of this node
	leaders map[uint64]*EpochLeaders // pre-computed leader selection for each epoch
}

func NewConsensusCommittee(
	state protocol.ReadOnlyState,
	me flow.Identifier,
) (*Consensus, error) {

	com := &Consensus{
		state:   state,
		me:      me,
		leaders: make(map[uint64]*EpochLeaders),
	}

	// pre-compute leader selection for current epoch
	epoch := state.Final().Epochs().Current()
	err := com.computeLeaderSelection(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for epoch: %w", err)
	}

	return com, nil
}

func (c *Consensus) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	identities, err := c.state.AtBlockID(blockID).Identities(filter.And(
		consensusMemberFilter,
		selector,
	))
	return identities, err
}

func (c *Consensus) Identity(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
	identities, err := c.Identities(blockID, filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identity (id=%s): %w", nodeID, err)
	}
	if len(identities) != 1 {
		return nil, fmt.Errorf("found invalid number (%d) of identities for node id (%s)", len(identities), nodeID)
	}
	return identities[0], nil
}

func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	// STEP 1 - look for an epoch matching this view for which we have already
	// pre-computed leader selection. Epochs last ~500k views, so we find the
	// epoch here 99.99% of the time.
	for _, epoch := range c.leaders {
		if view >= epoch.selection.FirstView() && view <= epoch.selection.FinalView() {
			index, err := epoch.selection.LeaderIndexForView(view)
			if err != nil {
				return flow.ZeroID, fmt.Errorf("could not get leader index for view %d: %w", view, err)
			}
			return epoch.identities[index].NodeID, nil
		}
	}

	// STEP 2 - pre-compute leader selection for next epoch, then proceed under
	// the assumption that the view is from the next epoch.
	next := c.state.Final().Epochs().Next()
	err := c.computeLeaderSelection(next)
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
	dkg, err := c.state.AtBlockID(blockID).Epochs().Current().DKG()
	return dkg, err
}

// computeLeaderSelection pre-computes and stores the leader selection for the
// given epoch.
func (c *Consensus) computeLeaderSelection(epoch protocol.Epoch) error {

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
	selection, err := leader.SelectionForEpoch(epoch)
	if err != nil {
		return fmt.Errorf("could not get leader selection for current epoch: %w", err)
	}

	c.leaders[counter] = &EpochLeaders{
		selection:  selection,
		identities: identities.Filter(consensusMemberFilter),
	}
	return nil
}
