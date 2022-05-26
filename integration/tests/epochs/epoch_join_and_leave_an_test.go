package epochs

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

func TestEpochJoinAndLeaveAN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_RESOURCE_INTENSIVE, "epochs join/leave tests should be run on an machine with adequate resources")
	suite.Run(t, new(EpochJoinAndLeaveANSuite))
}

type EpochJoinAndLeaveANSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveAN should update access nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveANSuite) TestEpochJoinAndLeaveAN() {
	s.runTestEpochJoinAndLeave(flow.RoleAccess, s.assertNetworkHealthyAfterANChange)
}
