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
	status, err := collector.ProcessApproval(approval)
	require.False(s.T(), status.SufficientApprovalsForSealing)
	require.Nil(s.T(), status.AggregatedSignatures)
	require.NoError(s.T(), err)
}

func (s *ApprovalCollectorTestSuite) TestProcessApproval_CollectSealingRecord() {
	collector := NewApprovalCollector(s.incorporatedResult, s.chunkAssignment, s.authorizedVerifiers, uint(len(s.authorizedVerifiers)))
	expectedSignatures := make([]flow.AggregatedSignature, s.incorporatedResult.Result.Chunks.Len())
	for i := 0; i < s.chunks.Len(); i++ {
		chunk := s.chunks[i]
		var status *SealingRecord
		var err error
		sigCollector := flow.NewSignatureCollector()
		for verID, _ := range s.authorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			status, err = collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
			sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
		}
		expectedSignatures[i] = sigCollector.ToAggregatedSignature()

		if i == s.chunks.Len()-1 {
			require.True(s.T(), status.SufficientApprovalsForSealing)
			require.ElementsMatch(s.T(), expectedSignatures, status.AggregatedSignatures)
		} else {
			require.False(s.T(), status.SufficientApprovalsForSealing)
		}
	}
}
