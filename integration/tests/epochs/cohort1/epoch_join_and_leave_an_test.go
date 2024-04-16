package cohort1

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEpochJoinAndLeaveAN(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_TODO, "kvstore: sealing segment doesn't support multiple protocol states")
	suite.Run(t, new(EpochJoinAndLeaveANSuite))
}

type EpochJoinAndLeaveANSuite struct {
	epochs.DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveAN should update access nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveANSuite) TestEpochJoinAndLeaveAN() {
	s.RunTestEpochJoinAndLeave(flow.RoleAccess, s.AssertNetworkHealthyAfterANChange)
}
