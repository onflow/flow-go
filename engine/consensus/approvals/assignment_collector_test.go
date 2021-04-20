package approvals

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAssignmentCollector tests behavior of AssignmentCollector in different scenarios
// AssignmentCollector is responsible collecting approvals that satisfy one assignment, meaning that we will
// have multiple collectors for one execution result as same result can be incorporated in multiple forks.
// AssignmentCollector has a strict ordering of processing, before processing approvals at least one incorporated result has to be
// processed.
// AssignmentCollector takes advantage of internal caching to speed up processing approvals for different assignments
// AssignmentCollector is responsible for validating approvals on result-level(checking signature, identity).
func TestAssignmentCollector(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTestSuite))
}

type AssignmentCollectorTestSuite struct {
	BaseApprovalsTestSuite

	blocks               map[flow.Identifier]*flow.Header
	state                *protocol.State
	assigner             *module.ChunkAssigner
	sealsPL              *mempool.IncorporatedResultSeals
	sigVerifier          *module.Verifier
	conduit              *mocknetwork.Conduit
	identitiesCache      map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	requestTracker       *sealing.RequestTracker
	getCachedBlockHeight GetCachedBlockHeight

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.conduit = &mocknetwork.Conduit{}

	s.requestTracker = sealing.NewRequestTracker(1, 3)

	// setup blocks cache for protocol state
	s.blocks = make(map[flow.Identifier]*flow.Header)
	s.blocks[s.Block.ID()] = &s.Block
	s.blocks[s.IncorporatedBlock.ID()] = &s.IncorporatedBlock

	// setup identities for each block
	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(func(result *flow.ExecutionResult, blockID flow.Identifier) *chunks.Assignment {
		return s.ChunksAssignment
	}, func(result *flow.ExecutionResult, blockID flow.Identifier) error { return nil })

	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if block, found := s.blocks[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[blockID])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)

	s.getCachedBlockHeight = func(blockID flow.Identifier) (uint64, error) {
		block, err := s.state.AtBlockID(blockID).Head()
		if err != nil {
			return 0, err
		}
		return block.Height, nil
	}

	var err error
	s.collector, err = NewAssignmentCollector(s.IncorporatedResult.Result, s.state, s.assigner, s.sealsPL,
		s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, uint(len(s.AuthorizedVerifiers)))
	require.NoError(s.T(), err)
}

// TestProcessAssignment_ApprovalsAfterResult tests a scenario when first we have discovered execution result
// and after that we started receiving approvals. In this scenario we should be able to create a seal right
// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_ApprovalsAfterResult() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err = s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporatedResult_ReusingCachedApprovals tests a scenario where we successfully processed approvals for one incorporated result
// and we are able to reuse those approvals for another incorporated result of same execution result
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult_ReusingCachedApprovals() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Twice()
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err = s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	// at this point we have proposed a seal, let's construct new incorporated result with same assignment
	// but different incorporated block ID resulting in new seal.
	incorporatedResult := unittest.IncorporatedResult.Fixture(unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))
	err = s.collector.ProcessIncorporatedResult(incorporatedResult)
	require.NoError(s.T(), err)
	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)

}

// TestProcessAssignment_InvalidSignature tests a scenario processing approval with invalid signature
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_InvalidSignature() {
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index), unittest.WithApproverID(s.VerID))
	err = s.collector.ProcessAssignment(approval)
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

		collector, err := NewAssignmentCollector(s.IncorporatedResult.Result, s.state, assigner, s.sealsPL,
			s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, 1)
		require.NoError(s.T(), err)

		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})

	s.Run("invalid-verifier-identities", func() {
		collector, err := NewAssignmentCollector(s.IncorporatedResult.Result, s.state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, 1)
		require.NoError(s.T(), err)
		// delete identities for Result.BlockID
		delete(s.identitiesCache, s.IncorporatedResult.Result.BlockID)
		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})
}

// TestProcessIncorporatedResult_InvalidIdentity tests a few scenarios where verifier identity is not correct
// by one or another reason
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult_InvalidIdentity() {

	s.Run("verifier-not-staked", func() {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
		identity.Stake = 0 // invalid stake

		state := &protocol.State{}
		state.On("AtBlockID", mock.Anything).Return(
			func(blockID flow.Identifier) realproto.Snapshot {
				return unittest.StateSnapshotForKnownBlock(
					&s.Block,
					map[flow.Identifier]*flow.Identity{identity.NodeID: identity},
				)
			},
		)

		collector, err := NewAssignmentCollector(s.IncorporatedResult.Result, state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, 1)
		require.NoError(s.T(), err)
		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
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

		collector, err := NewAssignmentCollector(s.IncorporatedResult.Result, state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, 1)
		require.NoError(s.T(), err)
		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
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

		collector, err := NewAssignmentCollector(s.IncorporatedResult.Result, state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, s.requestTracker, s.getCachedBlockHeight, 1)
		require.NoError(s.T(), err)
		err = collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})
}

// TestProcessAssignment_BeforeIncorporatedResult tests scenario when approval is submitted before execution result
// is discovered, without execution result we are missing information for verification. Calling `ProcessAssignment` before `ProcessApproval`
// should result in error
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_BeforeIncorporatedResult() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index), unittest.WithApproverID(s.VerID))
	err := s.collector.ProcessAssignment(approval)
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

		s.blocks[incorporatedBlock.ID()] = &incorporatedBlock
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
	s.conduit.On("Publish", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Run(func(args mock.Arguments) {
			// collect the request
			ar, ok := args[0].(*messages.ApprovalRequest)
			s.Assert().True(ok)
			requests = append(requests, ar)
		})

	err := s.collector.RequestMissingApprovals(lastHeight)
	require.NoError(s.T(), err)

	// first time it goes through, no requests should be made because of the
	// blackout period
	require.Len(s.T(), requests, 0)

	// wait for the max blackout period to elapse and retry
	time.Sleep(3 * time.Second)

	// requesting with immature height will be ignored
	err = s.collector.RequestMissingApprovals(lastHeight - uint64(len(incorporatedBlocks)) - 1)
	s.Require().NoError(err)
	require.Len(s.T(), requests, 0)

	err = s.collector.RequestMissingApprovals(lastHeight)
	s.Require().NoError(err)

	require.Len(s.T(), requests, s.Chunks.Len()*len(s.collector.collectors))

	resultID := s.IncorporatedResult.Result.ID()
	for _, chunk := range s.Chunks {
		for _, incorporatedResult := range incorporatedResults {
			requestItem := s.requestTracker.Get(resultID, incorporatedResult.IncorporatedBlockID, chunk.Index)
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
	err = s.collector.CheckEmergencySealing(s.IncorporatedBlock.Height)
	require.NoError(s.T(), err)

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err = s.collector.CheckEmergencySealing(sealing.DefaultEmergencySealingThreshold + s.IncorporatedBlock.Height)
	require.NoError(s.T(), err)

	s.sealsPL.AssertExpectations(s.T())
}
