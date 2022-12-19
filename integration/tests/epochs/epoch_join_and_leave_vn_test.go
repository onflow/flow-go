package epochs

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
)

func TestEpochJoinAndLeaveVN(t *testing.T) {
	// TODO this test is blocked by https://github.com/dapperlabs/flow-go/issues/6443
	//unittest.SkipUnless(t, unittest.TEST_FLAKY, "this test is flaky due to an unhandled case in service event processing following epoch transition https://github.com/dapperlabs/flow-go/issues/6443")
	suite.Run(t, new(EpochJoinAndLeaveVNSuite))
}

type EpochJoinAndLeaveVNSuite struct {
	DynamicEpochTransitionSuite
}

func (s *EpochJoinAndLeaveVNSuite) SetupTest() {
	// require approvals for seals to verify that the joining VN is producing valid seals in the second epoch
	s.RequiredSealApprovals = 1
	// increase epoch length to account for greater sealing lag due to above
	s.StakingAuctionLen = 200
	s.DKGPhaseLen = 100
	s.EpochLen = 650
	s.EpochCommitSafetyThreshold = 50
	s.DynamicEpochTransitionSuite.SetupTest()
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveVNSuite) TestEpochJoinAndLeaveVN() {
	s.runTestEpochJoinAndLeave(flow.RoleVerification, s.assertNetworkHealthyAfterVNChange)
}
