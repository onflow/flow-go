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
	"github.com/onflow/flow-go/state/protocol/seed"
)

var errSelectionNotComputed = fmt.Errorf("leader selection for epoch not yet computed")

// Consensus represents the main committee for consensus nodes. The consensus
// committee persists across epochs.
type Consensus struct {
	mu      sync.RWMutex
	state   protocol.State                     // the protocol state
	me      flow.Identifier                    // the node ID of this node
	leaders map[uint64]*leader.LeaderSelection // pre-computed leader selection for each epoch
}

var _ hotstuff.Committee = (*Consensus)(nil)

func NewConsensusCommittee(state protocol.State, me flow.Identifier) (*Consensus, error) {

	com := &Consensus{
		state:   state,
		me:      me,
		leaders: make(map[uint64]*leader.LeaderSelection),
	}

	final := state.Final()

	// pre-compute leader selection for current epoch
	current := final.Epochs().Current()
	_, err := com.prepareLeaderSelection(current)
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

	_, err = com.prepareLeaderSelection(previous)
	if err != nil {
		return nil, fmt.Errorf("could not add leader for previous epoch: %w", err)
	}

	return com, nil
}

func (c *Consensus) Identities(blockID flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	il, err := c.state.AtBlockID(blockID).Identities(filter.And(
		filter.IsVotingConsensusCommitteeMember,
		selector,
	))
	return il, err
}

func (c *Consensus) Identity(blockID flow.Identifier, nodeID flow.Identifier) (*flow.Identity, error) {
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

// LeaderForView returns the node ID of the leader for the given view. Returns
// the following errors:
//  * epoch containing the requested view has not been set up (protocol.ErrNextEpochNotSetup)
//  * epoch is too far in the past (leader.InvalidViewError)
//  * any other error indicates an unexpected internal error
func (c *Consensus) LeaderForView(view uint64) (flow.Identifier, error) {

	// try to retrieve the leader from a pre-computed LeaderSelection
	id, err := c.precomputedLeaderForView(view)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, errSelectionNotComputed) {
		return flow.ZeroID, err
	}
	// we only reach the following code, if we got a errSelectionNotComputed

	// STEP 2 - we haven't yet computed leader selection for an epoch containing
	// the requested view. We compute leader selection for the current and previous
	// epoch (w.r.t. the finalized head) at initialization then compute leader
	// selection for the next epoch when we encounter any view for which we don't
	// know the leader. The series of epochs we have computed leaders for is
	// strictly consecutive, meaning we know the leader for all views V where:
	//
	//   oldestEpoch.firstView <= V <= newestEpoch.finalView
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
	epochs := c.state.Final().Epochs()
	next := epochs.Next()

	// TMP: EMERGENCY EPOCH CHAIN CONTINUATION [EECC]
	//
	// If we reach this code-path, it means we are about to propose or vote
	// for the first block in the next epoch. If that epoch has not been
	// committed or set up, rather than stopping consensus, this intervention
	// will create a new fallback leader selection for the next epoch containing
	// 6 months worth of views, so that consensus will have leaders specified
	// for the duration of the current spork, without any epoch transitions.
	//
	_, err = next.DKG() // either of the following errors indicates that we have transitioned into EECC
	if errors.Is(err, protocol.ErrEpochNotCommitted) || errors.Is(err, protocol.ErrNextEpochNotSetup) {
		current := epochs.Current()

		currentCounter, err := current.Counter()
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get next epoch currentCounter: %w", err)
		}
		identities, err := current.InitialIdentities()
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get epoch initial identities: %w", err)
		}
		// Get the random source
		// CAUTION: this is re-using the same leader selection random source from the now-ending epoch
		randomSeed, err := current.RandomSource()
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get epoch seed: %w", err)
		}
		currentFinalView, err := current.FinalView()
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get epoch first view: %w", err)
		}

		// we will inject a fallback leader selection in place of the next epoch
		counter := currentCounter + 1
		// the fallback leader selection begins after the final view of the current epoch
		firstView := currentFinalView + 1

		// create random number generator from the seed and customizer
		rng, err := seed.PRGFromRandomSource(randomSeed, seed.ProtocolConsensusLeaderSelection)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not create rng from seed: %w", err)
		}

		selection, err := leader.ComputeLeaderSelection(
			firstView,
			rng,
			int(firstView+leader.EstimatedSixMonthOfViews), // the fallback epoch lasts until the next spork
			identities.Filter(filter.IsVotingConsensusCommitteeMember),
		)
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not compute epoch fallback leader selection: %w", err)
		}
		c.mu.Lock()
		c.leaders[counter] = selection
		c.mu.Unlock()
		return selection.LeaderForView(view)
	}
	if err != nil {
		return flow.ZeroID, fmt.Errorf("unexpected error in EECC logic while retrieving DKG data: %w", err)
	}

	// HAPPY PATH logic
	selection, err := c.prepareLeaderSelection(next)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not compute leader selection for next epoch: %w", err)
	}

	return selection.LeaderForView(view)
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
//   * errSelectionNotComputed [sentinel error] if there is no Epoch for view stored in `c.leaders`
//   * unspecific error in case of unexpected problems and bugs
func (c *Consensus) precomputedLeaderForView(view uint64) (flow.Identifier, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// STEP 1 - look for an epoch matching this view for which we have already
	// pre-computed leader selection. Epochs last ~500k views, so we find the
	// epoch here 99.99% of the time. Since epochs are long-lived, it is fine
	// for this to be linear in the number of epochs we have observed.
	for _, selection := range c.leaders {

		// try retrieving the leader
		leaderID, err := selection.LeaderForView(view)
		// if the view is out of range, try the next epoch
		if leader.IsInvalidViewError(err) {
			continue
		}
		if err != nil {
			return flow.ZeroID, fmt.Errorf("could not get leader: %w", err)
		}

		return leaderID, nil
	}

	return flow.ZeroID, errSelectionNotComputed
}

// prepareLeaderSelection pre-computes and stores the leader selection for the
// given epoch. Computing leader selection for the same epoch multiple times
// is a no-op.
//
// Returns the leader selection for the given epoch.
func (c *Consensus) prepareLeaderSelection(epoch protocol.Epoch) (*leader.LeaderSelection, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get counter for current epoch: %w", err)
	}
	// this is a no-op if we have already computed leaders for this epoch
	selection, exists := c.leaders[counter]
	if exists {
		return selection, nil
	}

	selection, err = leader.SelectionForConsensus(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not get leader selection for current epoch: %w", err)
	}
	c.leaders[counter] = selection

	// now prune any old epochs, if we have exceeded our maximum of 3
	// if we have fewer than 3 epochs, this is a no-op

	// find the maximum counter, including the epoch we just computed
	max := uint64(0)
	for counter := range c.leaders {
		if counter > max {
			max = counter
		}
	}

	// remove any epochs which aren't within the most recent 3
	for counter := range c.leaders {
		if counter+3 <= max {
			delete(c.leaders, counter)
		}
	}

	return selection, nil
}
