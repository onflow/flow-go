package approvals

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	realmodule "github.com/onflow/flow-go/module"
	realmempool "github.com/onflow/flow-go/module/mempool"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAssignmentCollector tests behavior of AssignmentCollector in different scenarios
// AssignmentCollector is responsible collecting approvals that satisfy one assignment, meaning that we will
// have multiple collectorTree for one execution result as same result can be incorporated in multiple forks.
// AssignmentCollector has a strict ordering of processing, before processing approvals at least one incorporated result has to be
// processed.
// AssignmentCollector takes advantage of internal caching to speed up processing approvals for different assignments
// AssignmentCollector is responsible for validating approvals on result-level(checking signature, identity).
func TestAssignmentCollector(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTestSuite))
}

func newVerifyingAssignmentCollector(logger zerolog.Logger,
	workerPool *workerpool.WorkerPool,
	result *flow.ExecutionResult,
	state realproto.State,
	headers realstorage.Headers,
	assigner realmodule.ChunkAssigner,
	seals realmempool.IncorporatedResultSeals,
	sigHasher hash.Hasher,
	approvalConduit network.Conduit,
	requestTracker *RequestTracker,
	requiredApprovalsForSealConstruction uint,
) (*VerifyingAssignmentCollector, error) {
	b, err := NewAssignmentCollectorBase(logger, workerPool, result, state, headers, assigner, seals, sigHasher,
		approvalConduit, requestTracker, requiredApprovalsForSealConstruction)
	if err != nil {
		return nil, err
	}
	return NewVerifyingAssignmentCollector(b)
}

type AssignmentCollectorTestSuite struct {
	BaseAssignmentCollectorTestSuite
	collector *VerifyingAssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	var err error
	s.collector, err = newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, s.State, s.Headers,
		s.Assigner, s.SealsPL, s.SigHasher, s.Conduit, s.RequestTracker, uint(len(s.AuthorizedVerifiers)))
	require.NoError(s.T(), err)
}

// TestProcessApproval_ApprovalsAfterResult tests a scenario when first we have discovered execution result
// and after that we started receiving approvals. In this scenario we should be able to create a seal right
// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (s *AssignmentCollectorTestSuite) TestProcessApproval_ApprovalsAfterResult() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.SealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			require.Equal(s.T(), s.Block.ID(), seal.Seal.BlockID)
			require.Equal(s.T(), s.IncorporatedResult.Result.ID(), seal.Seal.ResultID)
		},
	).Return(true, nil).Once()
	s.PublicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	blockID := s.Block.ID()
	resultID := s.IncorporatedResult.Result.ID()
	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(blockID),
				unittest.WithExecutionResultID(resultID))
			err = s.collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	s.SealsPL.AssertExpectations(s.T())
}

// TestProcessIncorporatedResult_ReusingCachedApprovals tests a scenario where we successfully processed approvals for one incorporated result
// and we are able to reuse those approvals for another incorporated result of same execution result
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult_ReusingCachedApprovals() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.SealsPL.On("Add", mock.Anything).Return(true, nil).Twice()
	s.PublicKey.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	blockID := s.Block.ID()
	resultID := s.IncorporatedResult.Result.ID()
	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(blockID),
				unittest.WithExecutionResultID(resultID))
			err = s.collector.ProcessApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	incorporatedBlock := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.Blocks[incorporatedBlock.ID()] = &incorporatedBlock

	// at this point we have proposed a seal, let's construct new incorporated result with same assignment
	// but different incorporated block ID resulting in new seal.
	incorporatedResult := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(incorporatedBlock.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result),
	)

	err = s.collector.ProcessIncorporatedResult(incorporatedResult)
	require.NoError(s.T(), err)
	s.SealsPL.AssertExpectations(s.T())

}

// TestProcessApproval_InvalidSignature tests a scenario processing approval with invalid signature
func (s *AssignmentCollectorTestSuite) TestProcessApproval_InvalidSignature() {

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithApproverID(s.VerID),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))

	// attestation signature is valid
	s.PublicKey.On("Verify", mock.Anything, approval.Body.AttestationSignature, mock.Anything).Return(true, nil).Once()
	// approval signature is invalid
	s.PublicKey.On("Verify", mock.Anything, approval.VerifierSignature, mock.Anything).Return(false, nil).Once()

	err = s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}

// TestProcessApproval_InvalidBlockID tests a scenario processing approval with invalid block ID
func (s *AssignmentCollectorTestSuite) TestProcessApproval_InvalidBlockID() {

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithApproverID(s.VerID),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))

	err = s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}

// TestProcessApproval_InvalidBlockChunkIndex tests a scenario processing approval with invalid chunk index
func (s *AssignmentCollectorTestSuite) TestProcessApproval_InvalidBlockChunkIndex() {

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	approval := unittest.ResultApprovalFixture(unittest.WithChunk(uint64(s.Chunks.Len())),
		unittest.WithApproverID(s.VerID),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))

	err = s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}

// TestProcessIncorporatedResult tests different scenarios for processing incorporated result
// Expected to process valid incorporated result without error and reject invalid incorporated results
// with engine.InvalidInputError
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult() {
	s.Run("valid-incorporated-result", func() {
		err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.NoError(s.T(), err)
	})

	s.Run("invalid-assignment", func() {
		assigner := &module.ChunkAssigner{}
		assigner.On("Assign", mock.Anything, mock.Anything).Return(nil, fmt.Errorf(""))

		collector, err := newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, s.State, s.Headers,
			assigner, s.SealsPL, s.SigHasher, s.Conduit, s.RequestTracker, 1)
		require.NoError(s.T(), err)

		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
	})

	s.Run("invalid-verifier-identities", func() {
		// delete identities for Result.BlockID
		delete(s.IdentitiesCache, s.IncorporatedResult.Result.BlockID)
		s.Snapshots[s.IncorporatedResult.Result.BlockID] = unittest.StateSnapshotForKnownBlock(&s.Block, nil)
		collector, err := newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, s.State, s.Headers,
			s.Assigner, s.SealsPL, s.SigHasher, s.Conduit, s.RequestTracker, 1)
		require.Error(s.T(), err)
		require.Nil(s.T(), collector)
	})
}

// TestProcessIncorporatedResult_InvalidIdentity tests a few scenarios where verifier identity is not correct
// by one or another reason
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult_InvalidIdentity() {

	s.Run("verifier zero-weight", func() {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		identity.Weight = 0 // zero weight

		state := &protocol.State{}
		state.On("AtBlockID", mock.Anything).Return(
			func(blockID flow.Identifier) realproto.Snapshot {
				return unittest.StateSnapshotForKnownBlock(
					&s.Block,
					map[flow.Identifier]*flow.Identity{identity.NodeID: identity},
				)
			},
		)

		collector, err := newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, state, s.Headers, s.Assigner, s.SealsPL,
			s.SigHasher, s.Conduit, s.RequestTracker, 1)
		require.Error(s.T(), err)
		require.Nil(s.T(), collector)
	})

	s.Run("verifier-ejected", func() {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		identity.Ejected = true // node ejected

		state := &protocol.State{}
		state.On("AtBlockID", mock.Anything).Return(
			func(blockID flow.Identifier) realproto.Snapshot {
				return unittest.StateSnapshotForKnownBlock(
					&s.Block,
					map[flow.Identifier]*flow.Identity{identity.NodeID: identity},
				)
			},
		)

		collector, err := newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, state, s.Headers, s.Assigner, s.SealsPL,
			s.SigHasher, s.Conduit, s.RequestTracker, 1)
		require.Nil(s.T(), collector)
		require.Error(s.T(), err)
	})
	s.Run("verifier-invalid-role", func() {
		// invalid role
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleAccess))

		state := &protocol.State{}
		state.On("AtBlockID", mock.Anything).Return(
			func(blockID flow.Identifier) realproto.Snapshot {
				return unittest.StateSnapshotForKnownBlock(
					&s.Block,
					map[flow.Identifier]*flow.Identity{identity.NodeID: identity},
				)
			},
		)

		collector, err := newVerifyingAssignmentCollector(unittest.Logger(), s.WorkerPool, s.IncorporatedResult.Result, state, s.Headers, s.Assigner, s.SealsPL,
			s.SigHasher, s.Conduit, s.RequestTracker, 1)
		require.Nil(s.T(), collector)
		require.Error(s.T(), err)
	})
}

// TestProcessApproval_BeforeIncorporatedResult tests scenario when approval is submitted before execution result
// is discovered, without execution result we are missing information for verification. Calling `ProcessApproval` before `ProcessApproval`
// should result in error
func (s *AssignmentCollectorTestSuite) TestProcessApproval_BeforeIncorporatedResult() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithApproverID(s.VerID),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))
	err := s.collector.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsInvalidInputError(err))
}

// TestRequestMissingApprovals checks that requests are sent only for chunks
// that have not collected enough approvals yet, and are sent only to the
// verifiers assigned to those chunks. It also checks that the threshold and
// rate limiting is respected.
func (s *AssignmentCollectorTestSuite) TestRequestMissingApprovals() {
	// build new assignment with 2 verifiers
	assignment := chunks.NewAssignment()
	for _, chunk := range s.Chunks {
		verifiers := s.ChunksAssignment.Verifiers(chunk)
		assignment.Add(chunk, verifiers[:2])
	}
	// replace old one
	s.ChunksAssignment = assignment

	incorporatedBlocks := make([]*flow.Header, 0)

	lastHeight := uint64(rand.Uint32())
	for i := 0; i < 2; i++ {
		incorporatedBlock := unittest.BlockHeaderFixture()
		incorporatedBlock.Height = lastHeight
		lastHeight++

		s.Blocks[incorporatedBlock.ID()] = &incorporatedBlock
		incorporatedBlocks = append(incorporatedBlocks, &incorporatedBlock)
	}

	incorporatedResults := make([]*flow.IncorporatedResult, 0, len(incorporatedBlocks))
	for _, block := range incorporatedBlocks {
		incorporatedResult := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result),
			unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()))
		incorporatedResults = append(incorporatedResults, incorporatedResult)

		err := s.collector.ProcessIncorporatedResult(incorporatedResult)
		require.NoError(s.T(), err)
	}

	requests := make([]*messages.ApprovalRequest, 0)
	// mock the Publish method when requests are sent to 2 verifiers
	s.Conduit.On("Publish", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			// collect the request
			ar, ok := args[0].(*messages.ApprovalRequest)
			s.Assert().True(ok)
			requests = append(requests, ar)
		})

	requestCount, err := s.collector.RequestMissingApprovals(&tracker.NoopSealingTracker{}, lastHeight)
	require.NoError(s.T(), err)

	// first time it goes through, no requests should be made because of the
	// blackout period
	require.Len(s.T(), requests, 0)
	require.Zero(s.T(), requestCount)

	// wait for the max blackout period to elapse and retry
	time.Sleep(3 * time.Second)

	// requesting with immature height will be ignored
	requestCount, err = s.collector.RequestMissingApprovals(&tracker.NoopSealingTracker{}, lastHeight-uint64(len(incorporatedBlocks))-1)
	s.Require().NoError(err)
	require.Len(s.T(), requests, 0)
	require.Zero(s.T(), requestCount)

	requestCount, err = s.collector.RequestMissingApprovals(&tracker.NoopSealingTracker{}, lastHeight)
	s.Require().NoError(err)

	require.Equal(s.T(), int(requestCount), s.Chunks.Len()*len(s.collector.collectors))
	require.Len(s.T(), requests, s.Chunks.Len()*len(s.collector.collectors))

	result := s.IncorporatedResult.Result
	for _, chunk := range s.Chunks {
		for _, incorporatedResult := range incorporatedResults {
			requestItem, _, err := s.RequestTracker.TryUpdate(result, incorporatedResult.IncorporatedBlockID, chunk.Index)
			require.NoError(s.T(), err)
			require.Equal(s.T(), uint(1), requestItem.Requests)
		}
	}
}

// TestCheckEmergencySealing tests that currently tracked incorporated results can be emergency sealed
// when height difference reached the emergency sealing threshold.
func (s *AssignmentCollectorTestSuite) TestCheckEmergencySealing() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	// checking emergency sealing with current height
	// should early exit without creating any seals
	err = s.collector.CheckEmergencySealing(&tracker.NoopSealingTracker{}, s.IncorporatedBlock.Height)
	require.NoError(s.T(), err)

	s.SealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			require.Equal(s.T(), s.Block.ID(), seal.Seal.BlockID)
			require.Equal(s.T(), s.IncorporatedResult.Result.ID(), seal.Seal.ResultID)
		},
	).Return(true, nil).Once()

	err = s.collector.CheckEmergencySealing(&tracker.NoopSealingTracker{}, DefaultEmergencySealingThreshold+s.IncorporatedBlock.Height)
	require.NoError(s.T(), err)

	s.SealsPL.AssertExpectations(s.T())
}
