package cohort3

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestPebbleExecutionStateSync(t *testing.T) {
	suite.Run(t, new(PebbleExecutionStateSync))
}

type PebbleExecutionStateSync struct {
	ExecutionStateSyncSuite
}

// TestPebbleDBHappyPath+ tests that Execution Nodes generate execution data, and Access Nodes are able to
// successfully sync the data to pebble DB
func (s *PebbleExecutionStateSync) TestPebbleDBHappyPath() {
	s.executionStateSyncTest()
}
