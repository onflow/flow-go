package epochs

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveLN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "epochs join/leave tests should be run on an machine with adequate resources")
	suite.Run(t, new(EpochJoinAndLeaveLNSuite))
}

type EpochJoinAndLeaveLNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveLN should update collection nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveLNSuite) TestEpochJoinAndLeaveLN() {
	s.runTestEpochJoinAndLeave(flow.RoleCollection, s.assertNetworkHealthyAfterLNChange)
}
