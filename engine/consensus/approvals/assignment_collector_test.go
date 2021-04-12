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

	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}

	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)

	// use same identities for incorporated and sealing block ID
	s.identitiesCache[s.IncorporatedResult.IncorporatedBlockID] = s.AuthorizedVerifiersIdentities
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiersIdentities

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

	s.collector = NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, uint(len(s.AuthorizedVerifiers)))
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
			err := s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessAssignment_ApprovalsBeforeResult tests a scenario when first we have received approvals for unknown
// execution result and after that we discovered execution result. In this scenario we should be able
// to create a seal right after discovering execution result since all approvals should be cached.(if cache capacity is big enough)
func (s *AssignmentCollectorTestSuite) TestProcessAssignment_ApprovalsBeforeResult() {
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index), unittest.WithApproverID(verID))
			err := s.collector.ProcessAssignment(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
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

		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, assigner, s.sealsPL, s.sigVerifier, 1)

		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})

	s.Run("invalid incorporated block identities", func() {
		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, 1)

		// delete identities for IncorporatedBlockID
		delete(s.identitiesCache, s.IncorporatedResult.IncorporatedBlockID)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})

	s.Run("invalid assignment identities", func() {
		collector := NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, 1)
		// delete identities for Result.BlockID
		delete(s.identitiesCache, s.IncorporatedResult.Result.BlockID)
		err := collector.ProcessIncorporatedResult(s.IncorporatedResult)
		require.Error(s.T(), err)
		require.True(s.T(), engine.IsInvalidInputError(err))
	})
}
