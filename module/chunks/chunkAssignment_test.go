package chunks_test

import (
	"github.com/stretchr/testify/suite"

	chunkmodule "github.com/dapperlabs/flow-go/module/chunks"
)

type ChunkAssignmentTestSuite struct {
	suite.Suite
	assignment *chunkmodule.ChunkAssignment
}

func (s *ChunkAssignmentTestSuite) SetupTest() {
}
