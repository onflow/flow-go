package epoch_join_and_leave_an

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/epochs"
	"github.com/stretchr/testify/suite"
)

func TestEpoch(t *testing.T) {
	suite.Run(t, new(epochs.EpochJoinAndLaveANSuite))
}
