package chunk_data_pack

import (
	"testing"

	"github.com/onflow/flow-go/integration/tests/execution"
	"github.com/stretchr/testify/suite"
)

func TestExecutionChunkDataPacks(t *testing.T) {
	suite.Run(t, new(execution.ChunkDataPacksSuite))
}
