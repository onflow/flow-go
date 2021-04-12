package approvals

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalCollector performs isolated testing of ApprovalCollector
// ApprovalCollector is responsible for delegating approval processing to ChunkApprovalCollector
// ApprovalCollector stores aggregated signatures for every chunk, once there is a signature for each chunk it is responsible
// for creating IncorporatedResultSeal and submitting it to the mempool.
// ApprovalCollector should reject approvals with invalid chunk index.
func TestApprovalCollector(t *testing.T) {
	suite.Run(t, new(ApprovalCollectorTestSuite))
}

type ApprovalCollectorTestSuite struct {
	BaseApprovalsTestSuite

	sealsPL   *mempool.IncorporatedResultSeals
	collector *ApprovalCollector
}

func (s *ApprovalCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.collector = NewApprovalCollector(s.IncorporatedResult, s.ChunksAssignment, s.sealsPL, s.AuthorizedVerifiers, uint(len(s.AuthorizedVerifiers)))
}

// TestProcessApproval_ValidApproval tests that valid approval is processed without error
func (s *ApprovalCollectorTestSuite) TestProcessApproval_ValidApproval() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index), unittest.WithApproverID(s.VerID))
	err := s.collector.ProcessApproval(approval)
	require.NoError(s.T(), err)
}

// TestProcessApproval_SealResult tests that after collecting enough approvals for every chunk ApprovalCollector will
// generate a seal and put it into seals mempool. This logic should be event driven and happen as soon as required threshold is
// met for each chunk.
func (s *ApprovalCollectorTestSuite) TestProcessApproval_SealResult() {
	expectedSignatures := make([]flow.AggregatedSignature, s.IncorporatedResult.Result.Chunks.Len())
	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	for i, chunk := range s.Chunks {
		var err error
		sigCollector := flow.NewSignatureCollector()
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err = s.collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
			sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
		}
		expectedSignatures[i] = sigCollector.ToAggregatedSignature()
	}

	finalState, _ := s.IncorporatedResult.Result.FinalStateCommitment()
	expectedArguments := &flow.IncorporatedResultSeal{
		IncorporatedResult: s.IncorporatedResult,
		Seal: &flow.Seal{
			BlockID:                s.IncorporatedResult.Result.BlockID,
			ResultID:               s.IncorporatedResult.Result.ID(),
			FinalState:             finalState,
			AggregatedApprovalSigs: expectedSignatures,
			ServiceEvents:          nil,
		},
	}

	s.sealsPL.AssertCalled(s.T(), "Add", expectedArguments)
}

// TestProcessApproval_InvalidChunk tests that approval with invalid chunk index will be rejected without
// processing.
func (s *ApprovalCollectorTestSuite) TestProcessApproval_InvalidChunk() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(uint64(s.Chunks.Len()+1)),
		unittest.WithApproverID(s.VerID))
	err := s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}
