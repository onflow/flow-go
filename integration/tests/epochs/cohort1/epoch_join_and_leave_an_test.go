package cohort1

import (
	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestEpochJoinAndLeaveAN(t *testing.T) {
	suite.Run(t, new(EpochJoinAndLeaveANSuite))
}

type EpochJoinAndLeaveANSuite struct {
	*epochs.DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveAN should update access nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveANSuite) TestEpochJoinAndLeaveAN() {
	s.RunTestEpochJoinAndLeave(flow.RoleAccess, s.AssertNetworkHealthyAfterANChange)
}
