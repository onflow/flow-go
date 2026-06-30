package approvals_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/testutil"
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
	testutil.BaseApprovalsTestSuite

	chunk           *flow.Chunk
	chunkAssignment map[flow.Identifier]struct{}
	collector       *approvals.ChunkApprovalCollector
}

func (s *ChunkApprovalCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()
	s.chunk = s.Chunks[0]
	verifiers, err := s.ChunksAssignment.Verifiers(s.chunk.Index)
	require.NoError(s.T(), err)
	s.chunkAssignment = verifiers
	s.collector = approvals.NewChunkApprovalCollector(s.chunkAssignment, uint(len(s.chunkAssignment)))
}

// TestProcessApproval_ValidApproval tests processing a valid approval. Expected to process it without error
// and report status to caller.
func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	require.Len(s.T(), s.collector.GetMissingSigners(), len(s.chunkAssignment))
	_, collected := s.collector.ProcessApproval(approval)
	require.False(s.T(), collected)
	require.Len(s.T(), s.collector.GetMissingSigners(), len(s.chunkAssignment)-1)
}

// TestProcessApproval_InvalidChunkAssignment tests processing approval with invalid chunk assignment. Expected to
// reject this approval, signature cache shouldn't be affected.
func (s *ChunkApprovalCollectorTestSuite) TestProcessApproval_InvalidChunkAssignment() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(s.VerID))
	delete(s.chunkAssignment, s.VerID)
	require.Len(s.T(), s.collector.GetMissingSigners(), len(s.chunkAssignment))
	_, collected := s.collector.ProcessApproval(approval)
	require.False(s.T(), collected)
	require.Len(s.T(), s.collector.GetMissingSigners(), len(s.chunkAssignment))
}

// TestGetAggregatedSignature_MultipleApprovals tests processing approvals from different verifiers. Expected to provide a valid
// aggregated sig that has `AttestationSignature` for every approval.
func (s *ChunkApprovalCollectorTestSuite) TestGetAggregatedSignature_MultipleApprovals() {
	var aggregatedSig flow.AggregatedSignature
	var collected bool
	sigCollector := approvals.NewSignatureCollector()
	require.Len(s.T(), s.collector.GetMissingSigners(), len(s.chunkAssignment))
	for verID := range s.AuthorizedVerifiers {
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunk.Index), unittest.WithApproverID(verID))
		aggregatedSig, collected = s.collector.ProcessApproval(approval)
		sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
	}

	require.True(s.T(), collected)
	require.NotNil(s.T(), aggregatedSig)
	require.Empty(s.T(), s.collector.GetMissingSigners())
	require.Equal(s.T(), sigCollector.ToAggregatedSignature(), aggregatedSig)
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
