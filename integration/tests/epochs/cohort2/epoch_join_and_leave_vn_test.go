package cohort2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestEpochJoinAndLeaveVN(t *testing.T) {
	s := new(EpochJoinAndLeaveVNSuite)

	suite.Run(t, &s.DynamicEpochTransitionSuite)
}

type EpochJoinAndLeaveVNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *EpochJoinAndLeaveVNSuite) SetupTest() {
	// require approvals for seals to verify that the joining VN is producing valid seals in the second epoch
	s.DynamicEpochTransitionSuite.RequiredSealApprovals = 1
	// slow down consensus, as sealing tends to lag behind
	s.DynamicEpochTransitionSuite.ConsensusProposalDuration = time.Millisecond * 250
	// increase epoch length to account for greater sealing lag due to above
	// NOTE: this value is set fairly aggressively to ensure shorter test times.
	// If flakiness due to failure to complete staking operations in time is observed,
	// try increasing (by 10-20 views).
	s.DynamicEpochTransitionSuite.StakingAuctionLen = 100
	s.DynamicEpochTransitionSuite.DKGPhaseLen = 100
	s.DynamicEpochTransitionSuite.EpochLen = 450
	s.DynamicEpochTransitionSuite.EpochCommitSafetyThreshold = 20
	s.DynamicEpochTransitionSuite.SetupTest()
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveVNSuite) TestEpochJoinAndLeaveVN() {
	s.DynamicEpochTransitionSuite.RunTestEpochJoinAndLeave(flow.RoleVerification, s.DynamicEpochTransitionSuite.AssertNetworkHealthyAfterVNChange)
}
