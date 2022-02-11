package epoch_join_and_leave_an

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/integration/tests/epochs"
)

func TestEpoch(t *testing.T) {
	suite.Run(t, new(epochs.EpochJoinAndLeaveVNSuite))
}
