package cohort3

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

func TestPebbleExecutionDataPruning(t *testing.T) {
	suite.Run(t, new(PebbleExecutionDataPruningSuite))
}

type PebbleExecutionDataPruningSuite struct {
	ExecutionDataPruningSuite
}

func (s *PebbleExecutionDataPruningSuite) SetupTest() {
	s.setup(execution_data.ExecutionDataDBModePebble)
}

// TestHappyPath tests the execution data pruning process using pebble DB in a happy path scenario.
func (s *PebbleExecutionDataPruningSuite) TestHappyPath() {
	s.executionDataPruningTest()
}
