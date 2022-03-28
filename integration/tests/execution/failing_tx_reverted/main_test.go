package failing_tx_reverted

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

func TestExecutionFailingTxReverted(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_FLAKY, "flaky")
	suite.Run(t, new(execution.FailingTxRevertedSuite))
}
