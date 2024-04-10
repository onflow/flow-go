package cohort2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveSN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: sealing segment doesn't support multiple protocol states")
	suite.Run(t, new(EpochJoinAndLeaveSNSuite))
}

type EpochJoinAndLeaveSNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

func (s *EpochJoinAndLeaveSNSuite) SetupTest() {
	// slow down the block rate. This is needed since the crypto module
	// update provides faster BLS operations.
	// TODO: fix the access integration test logic to function without slowing down
	// the block rate
	s.ConsensusProposalDuration = time.Millisecond * 250
	s.DynamicEpochTransitionSuite.SetupTest()
}

// TestEpochJoinAndLeaveSN should update consensus nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveSNSuite) TestEpochJoinAndLeaveSN() {
	s.RunTestEpochJoinAndLeave(flow.RoleConsensus, s.AssertNetworkHealthyAfterSNChange)
}
