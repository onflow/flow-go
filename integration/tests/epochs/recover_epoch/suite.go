package recover_epoch

import (
	"github.com/onflow/flow-go/integration/tests/epochs"
)

// Suite encapsulates common functionality for epoch integration tests.
type Suite struct {
	epochs.BaseSuite
}

func (s *Suite) SetupTest() {
	// use a longer staking auction length to accommodate staking operations for joining/leaving nodes
	// NOTE: this value is set fairly aggressively to ensure shorter test times.
	// If flakiness due to failure to complete staking operations in time is observed,
	// try increasing (by 10-20 views).
	s.StakingAuctionLen = 2
	s.DKGPhaseLen = 50
	s.EpochLen = 250
	s.EpochCommitSafetyThreshold = 20

	// run the generic setup, which starts up the network
	s.BaseSuite.SetupTest()
}
