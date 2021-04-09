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
	BaseApprovalsTestSuite
	chunk           *flow.Chunk
	chunkAssignment map[flow.Identifier]struct{}
}

func (s *ChunkApprovalCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()
	s.chunk = s.Chunks[0]
	s.chunkAssignment = make(map[flow.Identifier]struct{})
	for _, verifier := range s.ChunksAssignment.Verifiers(s.chunk) {
		s.chunkAssignment[verifier] = struct{}{}
	}
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.AuthorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	status := collector.ProcessApproval(approval)
	require.True(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(1), status.numberOfApprovals)
	require.Equal(s.T(), uint(1), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidChunkAssignment() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.AuthorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	delete(s.chunkAssignment, s.VerID)
	status := collector.ProcessApproval(approval)
	require.False(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(0), status.numberOfApprovals)
	require.Equal(s.T(), uint(0), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidVerifier() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.AuthorizedVerifiers)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	delete(s.AuthorizedVerifiers, s.VerID)
	status := collector.ProcessApproval(approval)
	require.False(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(0), status.numberOfApprovals)
	require.Equal(s.T(), uint(0), collector.chunkApprovals.NumberSignatures())
}

func (s *ChunkApprovalCollectorTestSuite) TestGetAggregatedSignature_MultipleApprovals() {
	collector := NewChunkApprovalCollector(s.chunkAssignment, s.AuthorizedVerifiers)
	var status ChunkProcessingStatus
	sigCollector := flow.NewSignatureCollector()
	for verID := range s.AuthorizedVerifiers {
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(verID))
		status = collector.ProcessApproval(approval)
		require.True(s.T(), status.approvalProcessed)
		sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
	}

	require.Equal(s.T(), uint(len(s.AuthorizedVerifiers)), status.numberOfApprovals)
	require.Equal(s.T(), sigCollector.ToAggregatedSignature(), collector.GetAggregatedSignature())
}
