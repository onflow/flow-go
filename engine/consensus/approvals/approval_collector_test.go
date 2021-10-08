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

	var err error
	s.collector, err = NewApprovalCollector(unittest.Logger(), s.IncorporatedResult, &s.IncorporatedBlock, &s.Block, s.ChunksAssignment, s.sealsPL, uint(len(s.AuthorizedVerifiers)))
	require.NoError(s.T(), err)
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
	s.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			require.Equal(s.T(), s.Block.ID(), seal.Seal.BlockID)
			require.Equal(s.T(), s.IncorporatedResult.Result.ID(), seal.Seal.ResultID)
			require.Equal(s.T(), s.IncorporatedResult.Result.BlockID, seal.Seal.BlockID)
			require.Equal(s.T(), seal.Seal.BlockID, seal.Header.ID())
		},
	).Return(true, nil).Once()

	for i, chunk := range s.Chunks {
		var err error
		sigCollector := NewSignatureCollector()
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err = s.collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
			sigCollector.Add(approval.Body.ApproverID, approval.Body.AttestationSignature)
		}
		expectedSignatures[i] = sigCollector.ToAggregatedSignature()
	}

	s.sealsPL.AssertExpectations(s.T())
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

// TestCollectMissingVerifiers tests that approval collector correctly assembles list of verifiers that haven't provided approvals
// for each chunk
func (s *ApprovalCollectorTestSuite) TestCollectMissingVerifiers() {
	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Maybe()

	assignedVerifiers := make(map[uint64]flow.IdentifierList)
	for _, chunk := range s.Chunks {
		assignedVerifiers[chunk.Index] = s.ChunksAssignment.Verifiers(chunk)
	}

	// no approvals processed
	for index, ids := range s.collector.CollectMissingVerifiers() {
		require.ElementsMatch(s.T(), ids, assignedVerifiers[index])
	}

	// process one approval for one each chunk
	for _, chunk := range s.Chunks {
		verID := assignedVerifiers[chunk.Index][0]
		approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
			unittest.WithApproverID(verID))
		err := s.collector.ProcessApproval(approval)
		require.NoError(s.T(), err)
	}

	for index, ids := range s.collector.CollectMissingVerifiers() {
		// skip first ID since we should have approval for it
		require.ElementsMatch(s.T(), ids, assignedVerifiers[index][1:])
	}

	// process remaining approvals for each chunk
	for _, chunk := range s.Chunks {
		for _, verID := range assignedVerifiers[chunk.Index] {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID))
			err := s.collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	// skip first ID since we should have approval for it
	require.Empty(s.T(), s.collector.CollectMissingVerifiers())
}
