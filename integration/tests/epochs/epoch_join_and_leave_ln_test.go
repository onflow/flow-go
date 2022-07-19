package epochs

import (
	"github.com/onflow/flow-go/utils/unittest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
)

func TestEpochJoinAndLeaveLN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "active-pacemaker, missing next epoch")
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
