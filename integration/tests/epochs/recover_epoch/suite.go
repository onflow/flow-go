package recover_epoch

import (
	"github.com/onflow/flow-go/integration/tests/epochs"
)

// Suite encapsulates common functionality for epoch integration tests.
type Suite struct {
	epochs.BaseSuite
}

func (s *Suite) SetupTest() {
	// use a shorter staking auction because we don't have staking operations in this case
	s.StakingAuctionLen = 2
	// to manually trigger EFM we assign very short dkg phase len ensuring the dkg will fail
	s.DKGPhaseLen = 10
	s.EpochLen = 80
	s.EpochCommitSafetyThreshold = 20
	s.NumOfCollectionClusters = 1

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}
