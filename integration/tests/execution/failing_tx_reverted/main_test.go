package failing_tx_reverted

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/stretchr/testify/suite"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	suite.Run(t, new(execution.FailingTxRevertedSuite))
}
