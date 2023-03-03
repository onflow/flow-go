package epochs

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveSN(t *testing.T) {
	suite.Run(t, new(EpochJoinAndLeaveSNSuite))
}

type EpochJoinAndLeaveSNSuite struct {
	DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveSN should update consensus nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveSNSuite) TestEpochJoinAndLeaveSN() {
	unittest.SkipUnless(s.T(), unittest.TEST_FLAKY, "fails on CI regularly")
	s.runTestEpochJoinAndLeave(flow.RoleConsensus, s.assertNetworkHealthyAfterSNChange)
}
