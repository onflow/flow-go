package epochs

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveVN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "epochs join/leave tests should be run on an machine with adequate resources")
	suite.Run(t, new(EpochJoinAndLeaveVNSuite))
	// this is a dummy update TODO: remove this before merging anything to master. AJM
}

type EpochJoinAndLeaveVNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveVN should update verification nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveVNSuite) TestEpochJoinAndLeaveVN() {
	s.runTestEpochJoinAndLeave(flow.RoleVerification, s.assertNetworkHealthyAfterVNChange)
}
