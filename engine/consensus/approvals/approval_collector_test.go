package approvals

import (
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
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

	sealsPL *mempool.IncorporatedResultSeals
}

func (s *ApprovalCollectorTestSuite) SetupTest() {
	s.sealsPL = &mempool.IncorporatedResultSeals{}

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
	collector := NewApprovalCollector(s.incorporatedResult, s.chunkAssignment, s.sealsPL, s.authorizedVerifiers, 3)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.chunks[0].Index), unittest.WithApproverID(s.verID))
	err := collector.ProcessApproval(approval)
	require.NoError(s.T(), err)
}

func (s *ApprovalCollectorTestSuite) TestProcessApproval_SealResult() {
	collector := NewApprovalCollector(s.incorporatedResult, s.chunkAssignment, s.sealsPL, s.authorizedVerifiers, uint(len(s.authorizedVerifiers)))
	expectedSignatures := make([]flow.AggregatedSignature, s.incorporatedResult.Result.Chunks.Len())

	s.sealsPL.On("Add", mock.Anything).Return(true, nil)

	for i := 0; i < s.chunks.Len(); i++ {
		chunk := s.chunks[i]
		var err error
		sigCollector := flow.NewSignatureCollector()
		for verID, _ := range s.authorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err = collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
			sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
		}
		expectedSignatures[i] = sigCollector.ToAggregatedSignature()
	}

	finalState, _ := s.incorporatedResult.Result.FinalStateCommitment()
	expectedArguments := &flow.IncorporatedResultSeal{
		IncorporatedResult: s.incorporatedResult,
		Seal: &flow.Seal{
			BlockID:                s.incorporatedResult.Result.BlockID,
			ResultID:               s.incorporatedResult.Result.ID(),
			FinalState:             finalState,
			AggregatedApprovalSigs: expectedSignatures,
			ServiceEvents:          nil,
		},
	}

	s.sealsPL.AssertCalled(s.T(), "Add", expectedArguments)
}

func (s *ApprovalCollectorTestSuite) TestProcessApproval_InvalidChunk() {
	collector := NewApprovalCollector(s.incorporatedResult, s.chunkAssignment, s.sealsPL, s.authorizedVerifiers, 3)
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(uint64(s.chunks.Len()+1)),
		unittest.WithApproverID(s.verID))
	err := collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}
