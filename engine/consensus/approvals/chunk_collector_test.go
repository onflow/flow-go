package approvals

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestChunkApprovalCollector(t *testing.T) {
	suite.Run(t, new(ChunkApprovalCollectorTestSuite))
}

type ChunkApprovalCollectorTestSuite struct {
	suite.Suite
	verID               flow.Identifier
	chunk               *flow.Chunk
	chunkAssignment     map[flow.Identifier]struct{}
	authorizedVerifiers map[flow.Identifier]struct{}
}

func (s *ChunkApprovalCollectorTestSuite) SetupTest() {
	blockID := unittest.IdentifierFixture()
	s.chunk = unittest.ChunkFixture(blockID, 0)

	verifiers := make(flow.IdentifierList, 0)
	s.authorizedVerifiers = make(map[flow.Identifier]struct{})
	s.chunkAssignment = make(map[flow.Identifier]struct{})
	for i := 0; i < 5; i++ {
		id := unittest.IdentifierFixture()
		verifiers = append(verifiers, id)
		s.authorizedVerifiers[id] = struct{}{}
		s.chunkAssignment[id] = struct{}{}
	}
	s.verID = verifiers[0]
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.authorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.verID))
	err := collector.ProcessApproval(approval)
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint(1), collector.chunkApprovals.NumberSignatures())
}
