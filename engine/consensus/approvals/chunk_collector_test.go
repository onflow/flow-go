package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
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
	status := collector.ProcessApproval(approval)
	require.True(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(1), status.numberOfApprovals)
	require.Equal(s.T(), uint(1), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidChunkAssignment() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.authorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.verID))
	delete(s.chunkAssignment, s.verID)
	status := collector.ProcessApproval(approval)
	require.False(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(0), status.numberOfApprovals)
	require.Equal(s.T(), uint(0), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidVerifier() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.authorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.verID))
	delete(s.authorizedVerifiers, s.verID)
	status := collector.ProcessApproval(approval)
	require.False(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(0), status.numberOfApprovals)
	require.Equal(s.T(), uint(0), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestGetAggregatedSignature_MultipleApprovals() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.authorizedVerifiers)
	var status ChunkProcessingStatus
	sigCollector := flow.NewSignatureCollector()
	for verID := range s.authorizedVerifiers {
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(verID))
		status = collector.ProcessApproval(approval)
		require.True(s.T(), status.approvalProcessed)
		sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
	}

	require.Equal(s.T(), uint(len(s.authorizedVerifiers)), status.numberOfApprovals)
	require.Equal(s.T(), sigCollector.ToAggregatedSignature(), collector.GetAggregatedSignature())
}
