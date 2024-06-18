package cohort1

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestEpochJoinAndLeaveLN(t *testing.T) {
	s := new(EpochJoinAndLeaveLNSuite)
	suite.Run(t, &s.DynamicEpochTransitionSuite)
}

type EpochJoinAndLeaveLNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveLN should update collection nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveLNSuite) TestEpochJoinAndLeaveLN() {
	s.DynamicEpochTransitionSuite.RunTestEpochJoinAndLeave(flow.RoleCollection, s.DynamicEpochTransitionSuite.AssertNetworkHealthyAfterLNChange)
}
