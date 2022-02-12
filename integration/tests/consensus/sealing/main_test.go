package sealing

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/consensus"
	"github.com/stretchr/testify/suite"
)

func TestExecutionStateSealing(t *testing.T) {
	suite.Run(t, new(consensus.SealingSuite))
}
