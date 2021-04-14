package approvals

import (
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/stretchr/testify/mock"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/engine"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestApprovalProcessingCore(t *testing.T) {
	suite.Run(t, new(ApprovalProcessingCoreTestSuite))
}

type ApprovalProcessingCoreTestSuite struct {
	BaseApprovalsTestSuite

	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	payloads        *storage.Payloads
	core            *approvalProcessingCore
}

func (c *ApprovalProcessingCoreTestSuite) SetupTest() {
	c.BaseApprovalsTestSuite.SetupTest()

	c.sealsPL = &mempool.IncorporatedResultSeals{}
	c.state = &protocol.State{}
	c.assigner = &module.ChunkAssigner{}
	c.sigVerifier = &module.Verifier{}

	c.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	c.identitiesCache[c.IncorporatedResult.Result.BlockID] = c.AuthorizedVerifiers

	c.assigner.On("Assign", mock.Anything, mock.Anything).Return(c.ChunksAssignment, nil)

	// define the protocol state snapshot for any block in `bc.Blocks`
	c.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if identities, found := c.identitiesCache[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(&c.Block, identities)
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)
	c.payloads = &storage.Payloads{}
	c.core = NewApprovalProcessingCore(c.payloads, c.state, c.assigner, c.sigVerifier, c.sealsPL, uint(len(c.AuthorizedVerifiers)))
}

func (c *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedApprovals() {
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(c.VerID),
		unittest.WithChunk(c.Chunks[0].Index),
		unittest.WithBlockID(c.Block.ID()))
	err := c.core.ProcessApproval(approval)
	require.NoError(c.T(), err)

	hotstuffBlock := hotmodel.Block{
		BlockID: c.Block.ID(),
	}

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(c.Block.ID()),
		unittest.Seal.WithResult(c.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	c.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()

	c.core.OnFinalizedBlock(&hotstuffBlock)

	err = c.core.ProcessApproval(approval)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsOutdatedInputError(err))
}
