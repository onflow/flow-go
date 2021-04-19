package approvals

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
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

	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	conduit         *mocknetwork.Conduit
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.conduit = &mocknetwork.Conduit{}

	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(s.ChunksAssignment, nil)

	// define the protocol state snapshot for any block in `bc.Blocks`
	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if identities, found := s.identitiesCache[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(&s.Block, identities)
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)

	s.collector = NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, s.conduit, uint(len(s.AuthorizedVerifiers)))
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, assigner, s.sealsPL,
			s.sigVerifier, s.conduit, 1)

		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})

	s.Run("invalid-verifier-identities", func() {
		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, 1)
		// delete identities for Result.BlockID
		delete(s.identitiesCache, s.IncorporatedResult.Result.BlockID)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, 1)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, 1)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), state, s.assigner, s.sealsPL,
			s.sigVerifier, s.conduit, 1)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
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
