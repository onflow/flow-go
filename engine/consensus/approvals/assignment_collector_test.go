package approvals

import (
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

	state       *protocol.State
	assigner    *module.ChunkAssigner
	sealsPL     *mempool.IncorporatedResultSeals
	sigVerifier *module.Verifier
	identities  map[flow.Identifier]*flow.Identity

	collector *AssignmentCollector
}

func (s *AssignmentCollectorTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.identities = make(map[flow.Identifier]*flow.Identity)

	identity := unittest.IdentityFixture()
	identity.NodeID = s.VerID
	s.identities[s.VerID] = identity

	// define the protocol state snapshot for any block in `bc.Blocks`
	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if s.IncorporatedResult.Result.BlockID != blockID {
				return unittest.StateSnapshotForUnknownBlock()
			}
			return unittest.StateSnapshotForKnownBlock(&s.Block, s.identities)
		},
	)

	s.collector = NewAssignmentCollector(s.IncorporatedResult.Result.ID(), s.state, s.assigner, s.sealsPL, s.sigVerifier, uint(len(s.AuthorizedVerifiers)))
}

func (s *AssignmentCollectorTestSuite) TestProcessAssignment_ValidApproval() {
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index), unittest.WithApproverID(s.VerID))
	err := s.collector.ProcessAssignment(approval)
	require.NoError(s.T(), err)
}

func (s *AssignmentCollectorTestSuite) TestProcessIncorporatedResult_ValidResult() {
	err := s.collector.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)
}
