package ln

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/onflow/flow-go/model/flow"
)

func TestEpochJoinAndLeaveLN(t *testing.T) {
	suite.Run(t, new(EpochJoinAndLeaveLNSuite))
}

type EpochJoinAndLeaveLNSuite struct {
	epochs.DynamicEpochTransitionSuite
}

// TestEpochJoinAndLeaveLN should update collection nodes and assert healthy network conditions
// after the epoch transition completes. See health check function for details.
func (s *EpochJoinAndLeaveLNSuite) TestEpochJoinAndLeaveLN() {
	s.RunTestEpochJoinAndLeave(flow.RoleCollection, s.AssertNetworkHealthyAfterLNChange)
}
