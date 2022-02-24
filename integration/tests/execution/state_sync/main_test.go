package state_sync

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/suite"
)

func TestExecutionStateSync(t *testing.T) {
	unittest.SkipUnless(t, unittest.TEST_DEPRECATED, "state sync disabled")
	suite.Run(t, new(execution.StateSyncSuite))
}
