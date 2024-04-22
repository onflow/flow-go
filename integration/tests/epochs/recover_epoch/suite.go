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
	s.DKGPhaseLen = 50
	s.EpochLen = 250
	s.EpochCommitSafetyThreshold = 20

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}
