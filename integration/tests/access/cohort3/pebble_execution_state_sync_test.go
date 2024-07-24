package cohort3

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

func TestPebbleExecutionStateSync(t *testing.T) {
	suite.Run(t, new(PebbleExecutionStateSync))
}

type PebbleExecutionStateSync struct {
	ExecutionStateSyncSuite
}

func (s *PebbleExecutionStateSync) SetupTest() {
	s.setup(execution_data.ExecutionDataDBModePebble)
}

// TestBadgerDBHappyPath tests that Execution Nodes generate execution data, and Access Nodes are able to
// successfully sync the data to pebble DB
func (s *PebbleExecutionStateSync) TestPebbleDBHappyPath() {
	s.executionStateSyncTest()
}
