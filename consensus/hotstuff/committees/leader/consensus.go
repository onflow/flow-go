package leader

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/seed"
)

// SelectionForConsensus pre-computes and returns leaders for the consensus committee
// in the given epoch. The consensus committee spans multiple epochs and the leader
// selection returned here is only valid for the input epoch, so it is necessary to
// call this for each upcoming epoch.
func SelectionForConsensus(epoch protocol.Epoch) (*LeaderSelection, error) {

	// pre-compute leader selection for the epoch
	identities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch initial identities: %w", err)
	}

	// get the epoch source of randomness
	randomSeed, err := epoch.RandomSource()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch seed: %w", err)
	}
	// create random number generator from the seed and customizer
	rng, err := seed.PRGFromRandomSource(randomSeed, seed.ProtocolConsensusLeaderSelection)
	if err != nil {
		return nil, fmt.Errorf("could not create rng: %w", err)
	}
	firstView, err := epoch.FirstView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch first view: %w", err)
	}
	finalView, err := epoch.FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch final view: %w", err)
	}

	leaders, err := ComputeLeaderSelection(
		firstView,
		rng,
		int(finalView-firstView+1), // add 1 because both first/final view are inclusive
		identities.Filter(filter.IsVotingConsensusCommitteeMember),
	)
	return leaders, err
}
