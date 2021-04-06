package approvals

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

func TestApprovalCollector(t *testing.T) {
	suite.Run(t, new(ApprovalCollectorTestSuite))
}

type ApprovalCollectorTestSuite struct {
	suite.Suite
	verID               flow.Identifier
	chunks              flow.ChunkList
	chunkAssignment     *chunks.Assignment
	authorizedVerifiers map[flow.Identifier]struct{}
	incorporatedResult  *flow.IncorporatedResult
}

func (s *ApprovalCollectorTestSuite) SetupTest() {
	blockID := unittest.IdentifierFixture()
	verifiers := make(flow.IdentifierList, 0)
	s.authorizedVerifiers = make(map[flow.Identifier]struct{})
	s.chunkAssignment = chunks.NewAssignment()
	s.chunks = unittest.ChunkListFixture(50, blockID)

	for j := 0; j < 5; j++ {
		id := unittest.IdentifierFixture()
		verifiers = append(verifiers, id)
		s.authorizedVerifiers[id] = struct{}{}
	}

	for _, chunk := range s.chunks {
		s.chunkAssignment.Add(chunk, verifiers)
	}

	s.verID = verifiers[0]
	result := unittest.ExecutionResultFixture()
	result.BlockID = blockID
	result.Chunks = s.chunks
	s.incorporatedResult = unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(result))
}

func (s *ApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	collector := NewApprovalCollector(s.incorporatedResult, s.chunkAssignment, s.authorizedVerifiers, 3)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunks[0].Index), unittest.WithApproverID(s.verID))
	err := collector.ProcessApproval(approval)
	require.NoError(s.T(), err)
	//require.True(s.T(), status.approvalProcessed)
	//require.Equal(s.T(), uint(1), status.numberOfApprovals)
	//require.Equal(s.T(), uint(1), collector.chunkApprovals.NumberSignatures())
}
