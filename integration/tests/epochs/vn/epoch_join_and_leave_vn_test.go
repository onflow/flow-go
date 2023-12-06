package vn

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/integration/tests/epochs"
)

func TestEpochJoinAndLeaveVN(t *testing.T) {
	suite.Run(t, new(EpochJoinAndLeaveVNSuite))
}

type EpochJoinAndLeaveVNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *EpochJoinAndLeaveVNSuite) SetupTest() {
	// require approvals for seals to verify that the joining VN is producing valid seals in the second epoch
	s.RequiredSealApprovals = 1
	// slow down consensus, as sealing tends to lag behind
	s.ConsensusProposalDuration = time.Millisecond * 250
	// increase epoch length to account for greater sealing lag due to above
	// NOTE: this value is set fairly aggressively to ensure shorter test times.
	// If flakiness due to failure to complete staking operations in time is observed,
	// try increasing (by 10-20 views).
	s.StakingAuctionLen = 100
	s.DKGPhaseLen = 100
	s.EpochLen = 450
	s.EpochCommitSafetyThreshold = 20
	s.Suite.SetupTest()
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveVNSuite) TestEpochJoinAndLeaveVN() {
	s.RunTestEpochJoinAndLeave(flow.RoleVerification, s.AssertNetworkHealthyAfterVNChange)
}
