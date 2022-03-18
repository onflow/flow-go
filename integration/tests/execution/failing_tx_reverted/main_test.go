package failing_tx_reverted

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/stretchr/testify/suite"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	// # TODO: confirm if this is the correct reason
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky")
	suite.Run(t, new(execution.FailingTxRevertedSuite))
}
