package approvals

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkApprovalCollector performs isolated testing of ChunkApprovalCollector.
// ChunkApprovalCollector has to process and cache signatures for result approvals that satisfy assignment.
// ChunkApprovalCollector has to reject approvals with invalid assignment.
// ChunkApprovalCollector is responsible for properly accumulating signatures and creating aggregated signature when requested.
func TestChunkApprovalCollector(t *testing.T) {
	suite.Run(t, new(ChunkApprovalCollectorTestSuite))
}

type ChunkApprovalCollectorTestSuite struct {
	BaseApprovalsTestSuite

	chunk           *flow.Chunk
	chunkAssignment map[flow.Identifier]struct{}
	collector       *ChunkApprovalCollector
}

func (s *ChunkApprovalCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()
	s.chunk = s.Chunks[0]
	s.chunkAssignment = make(map[flow.Identifier]struct{})
	for _, verifier := range s.ChunksAssignment.Verifiers(s.chunk) {
		s.chunkAssignment[verifier] = struct{}{}
	}
	s.collector = NewChunkApprovalCollector(s.chunkAssignment)
}

// TestProcessApproval_ValidApproval tests processing a valid approval. Expected to process it without error
// and report status to caller.
func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	status := s.collector.ProcessApproval(approval)
	require.True(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(1), status.numberOfApprovals)
	require.Equal(s.T(), uint(1), s.collector.chunkApprovals.NumberSignatures())
}

// TestProcessApproval_InvalidChunkAssignment tests processing approval with invalid chunk assignment. Expected to
// reject this approval, signature cache shouldn't be affected.
func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidChunkAssignment() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	delete(s.chunkAssignment, s.VerID)
	status := s.collector.ProcessApproval(approval)
	require.False(s.T(), status.approvalProcessed)
	require.Equal(s.T(), uint(0), status.numberOfApprovals)
	require.Equal(s.T(), uint(0), s.collector.chunkApprovals.NumberSignatures())
}

// TestGetAggregatedSignature_MultipleApprovals tests processing approvals from different verifiers. Expected to provide a valid
// aggregated sig that has `AttestationSignature` for every approval.
func (s *ChunkApprovalCollectorTestSuite) TestGetAggregatedSignature_MultipleApprovals() {
	var status ChunkProcessingStatus
	sigCollector := flow.NewSignatureCollector()
	for verID := range s.AuthorizedVerifiers {
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(verID))
		status = s.collector.ProcessApproval(approval)
		require.True(s.T(), status.approvalProcessed)
		sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
	}

	require.Equal(s.T(), uint(len(s.AuthorizedVerifiers)), status.numberOfApprovals)
	require.Equal(s.T(), sigCollector.ToAggregatedSignature(), s.collector.GetAggregatedSignature())
}

// TestGetMissingSigners tests that missing signers returns correct IDs of approvers that haven't provided an approval
func (s *ChunkApprovalCollectorTestSuite) TestGetMissingSigners() {
	assignedSigners := make(flow.IdentifierList, 0, len(s.chunkAssignment))
	for id := range s.chunkAssignment {
		assignedSigners = append(assignedSigners, id)
	}
	require.ElementsMatch(s.T(), assignedSigners, s.collector.GetMissingSigners())

	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(assignedSigners[0]))
	s.collector.ProcessApproval(approval)

	require.ElementsMatch(s.T(), assignedSigners[1:], s.collector.GetMissingSigners())

	for verID := range s.AuthorizedVerifiers {
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(verID))
		s.collector.ProcessApproval(approval)
	}

	require.Empty(s.T(), s.collector.GetMissingSigners())
}
