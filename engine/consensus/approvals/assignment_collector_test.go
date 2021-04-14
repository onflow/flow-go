package approvals

import (
	"fmt"
	"github.com/onflow/flow-go/engine"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAssignmentCollector(t *testing.T) {
	suite.Run(t, new(AssignmentCollectorTestSuite))
}

type AssignmentCollectorTestSuite struct {
	BaseApprovalsTestSuite

	State           *protocol.State
	Assigner        *module.ChunkAssigner
	SealsPL         *mempool.IncorporatedResultSeals
	SigVerifier     *module.Verifier
	IdentitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.SealsPL = &mempool.IncorporatedResultSeals{}
	s.State = &protocol.State{}
	s.Assigner = &module.ChunkAssigner{}
	s.SigVerifier = &module.Verifier{}

	s.IdentitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.IdentitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.Assigner.On("Assign", mock.Anything, mock.Anything).Return(s.ChunksAssignment, nil)

	// define the protocol State snapshot for any block in `bc.Blocks`
	s.State.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if identities, found := s.IdentitiesCache[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(&s.Block, identities)
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)

	s.collector = NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.State, s.Assigner, s.SealsPL, s.SigVerifier, uint(len(s.AuthorizedVerifiers)))
}

// TestProcessAssignment_ApprovalsAfterResult tests a scenario when first we have discovered execution result
// and after that we started receiving approvals. In this scenario we should be able to create a seal right
// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_ApprovalsAfterResult() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.SealsPL.On("Add", mock.Anything).Return(true, nil).Once()
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err := s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	s.SealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessAssignment_ApprovalsBeforeResult tests a scenario when first we have received approvals for unknown
// execution result and after that we discovered execution result. In this scenario we should be able
// to create a seal right after discovering execution result since all approvals should be cached.(if cache capacity is big enough)
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_ApprovalsBeforeResult() {
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err := s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	s.SealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.SealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporatedResult tests different scenarios for processing incorporated result
// Expected to process valid incorporated result without error and reject invalid incorporated results
// with engine.InvalidInputError
func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult() {
	s.Run("valid incorporated result", func() {
		err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.NoError(s.T(), err)
	})

	s.Run("invalid assignment", func() {
		assigner := &module.ChunkAssigner{}
		assigner.On("Assign", mock.Anything, mock.Anything).Return(nil, fmt.Errorf(""))

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.State, assigner, s.SealsPL, s.SigVerifier, 1)

		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})

	s.Run("invalid verifier identities", func() {
		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.State, s.Assigner, s.SealsPL, s.SigVerifier, 1)
		// delete identities for Result.BlockID
		delete(s.IdentitiesCache, s.IncorporatedResult.Result.BlockID)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})
}

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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.Assigner, s.SealsPL, s.SigVerifier, 1)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.Assigner, s.SealsPL, s.SigVerifier, 1)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.Assigner, s.SealsPL, s.SigVerifier, 1)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})
}
