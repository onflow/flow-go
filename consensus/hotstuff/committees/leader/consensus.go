package leader

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/prg"
)

// SelectionForConsensusFromEpoch returns the leader selection for the input epoch.
// See [SelectionForConsensus] for additional details.
func SelectionForConsensusFromEpoch(epoch protocol.CommittedEpoch) (*LeaderSelection, error) {

	// get the epoch source of randomness
	randomSeed, err := epoch.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch seed: %w", err)
	}
	firstView := epoch.FirstView()
	finalView := epoch.FinalView()

	leaders, err := SelectionForConsensus(
		epoch.InitialIdentities(),
		randomSeed,
		firstView,
		finalView,
	)
	return leaders, err
}

// SelectionForConsensus pre-computes and returns leaders for the consensus committee
// in the given epoch. The consensus committee spans multiple epochs and the leader
// selection returned here is only valid for the input epoch, so it is necessary to
// call this for each upcoming epoch.
func SelectionForConsensus(initialIdentities flow.IdentitySkeletonList, randomSeed []byte, firstView, finalView uint64) (*LeaderSelection, error) {
	rng, err := prg.New(randomSeed, prg.ConsensusLeaderSelection, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create rng: %w", err)
	}
	leaders, err := ComputeLeaderSelection(
		firstView,
		rng,
		int(finalView-firstView+1), // add 1 because both first/final view are inclusive
		initialIdentities.Filter(filter.IsConsensusCommitteeMember),
	)
	return leaders, err
}
