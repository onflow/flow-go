package cohort2

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveSN(t *testing.T) {
	suite.Run(t, new(EpochJoinAndLeaveSNSuite))
}

type EpochJoinAndLeaveSNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *EpochJoinAndLeaveSNSuite) SetupTest() {
	s.DynamicEpochTransitionSuite.SetupTest()
}

// TestEpochJoinAndLeaveSN should update consensus nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveSNSuite) TestEpochJoinAndLeaveSN() {
	unittest.SkipUnless(s.T(), unittest.TEST_TODO, "requires changes to the DKG so we can produce a valid DKG IndexMap")
	s.RunTestEpochJoinAndLeave(flow.RoleConsensus, s.AssertNetworkHealthyAfterSNChange)
}
