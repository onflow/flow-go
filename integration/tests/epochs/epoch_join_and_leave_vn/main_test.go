package epoch_join_and_leave_vn

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/stretchr/testify/suite"
)

func TestEpoch(t *testing.T) {
	suite.Run(t, new(epochs.EpochJoinAndLeaveVNSuite))
}
