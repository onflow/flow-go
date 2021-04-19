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
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalProcessingCore performs testing of approval processing core
// approvalProcessingCore is responsible for delegating processing to assignment collectors for each separate execution result
// approvalProcessingCore performs height based checks and decides if approval or incorporated result has to be processed at all
// or rejected as outdated or unverifiable.
// approvalProcessingCore maintains a LRU cache of known approvals that cannot be verified at the moment/
func TestApprovalProcessingCore(t *testing.T) {
	suite.Run(t, new(ApprovalProcessingCoreTestSuite))
}

type ApprovalProcessingCoreTestSuite struct {
	BaseApprovalsTestSuite

	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	conduit         *mocknetwork.Conduit
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
	c.conduit = &mocknetwork.Conduit{}

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
	c.core = NewApprovalProcessingCore(c.payloads, c.state, c.assigner, c.sigVerifier, c.sealsPL, c.conduit, uint(len(c.AuthorizedVerifiers)))
}

// TestOnBlockFinalized_RejectOutdatedApprovals tests that approvals will be rejected as outdated
// for block that is already sealed
func (c *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedApprovals() {
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(c.VerID),
		unittest.WithChunk(c.Chunks[0].Index),
		unittest.WithBlockID(c.Block.ID()))
	err := c.core.ProcessApproval(approval)
	require.NoError(c.T(), err)

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(c.Block.ID()),
		unittest.Seal.WithResult(c.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	c.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()

	c.core.OnFinalizedBlock(c.Block.ID())

	err = c.core.ProcessApproval(approval)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectOutdatedExecutionResult tests that incorporated result will be rejected as outdated
// if the block which is targeted by execution result is already sealed.
func (c *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedExecutionResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(c.Block.ID()),
		unittest.Seal.WithResult(c.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	c.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()

	c.core.OnFinalizedBlock(c.Block.ID())

	err := c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectUnverifiableEntries tests that core will reject both execution results
// and approvals for blocks that we have no information about.
func (c *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectUnverifiableEntries() {
	c.IncorporatedResult.Result.BlockID = unittest.IdentifierFixture() // replace blockID with random one
	err := c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsUnverifiableInputError(err))

	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(c.VerID),
		unittest.WithChunk(c.Chunks[0].Index))

	err = c.core.ProcessApproval(approval)
	require.Error(c.T(), err)
	require.True(c.T(), engine.IsUnverifiableInputError(err))
}

// TestProcessIncorporated_ApprovalsBeforeResult tests a scenario when first we have received approvals for unknown
// execution result and after that we discovered execution result. In this scenario we should be able
// to create a seal right after discovering execution result since all approvals should be cached.(if cache capacity is big enough)
func (c *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsBeforeResult() {
	c.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range c.Chunks {
		for verID := range c.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(c.Block.ID()),
				unittest.WithExecutionResultID(c.IncorporatedResult.Result.ID()))
			err := c.core.ProcessApproval(approval)
			require.NoError(c.T(), err)
		}
	}

	c.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.NoError(c.T(), err)

	c.sealsPL.AssertCalled(c.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ApprovalsAfterResult tests a scenario when first we have discovered execution result
//// and after that we started receiving approvals. In this scenario we should be able to create a seal right
//// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (c *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsAfterResult() {
	c.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	c.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.NoError(c.T(), err)

	for _, chunk := range c.Chunks {
		for verID := range c.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(c.Block.ID()),
				unittest.WithExecutionResultID(c.IncorporatedResult.Result.ID()))
			err := c.core.ProcessApproval(approval)
			require.NoError(c.T(), err)
		}
	}

	c.sealsPL.AssertCalled(c.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ProcessingInvalidApproval tests that processing invalid approval when result is discovered
// is correctly handled in case of sentinel error
func (c *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ProcessingInvalidApproval() {
	// fail signature verification for first approval
	c.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	// generate approvals for first chunk
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(c.Chunks[0].Index),
		unittest.WithApproverID(c.VerID),
		unittest.WithBlockID(c.Block.ID()),
		unittest.WithExecutionResultID(c.IncorporatedResult.Result.ID()))

	// this approval has to be cached since execution result is not known yet
	err := c.core.ProcessApproval(approval)
	require.NoError(c.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.NoError(c.T(), err)
}

// TestProcessIncorporated_ApprovalVerificationException tests that processing invalid approval when result is discovered
// is correctly handled in case of exception
func (c *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalVerificationException() {
	// fail signature verification with exception
	c.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, fmt.Errorf("exception")).Once()

	// generate approvals for first chunk
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(c.Chunks[0].Index),
		unittest.WithApproverID(c.VerID),
		unittest.WithBlockID(c.Block.ID()),
		unittest.WithExecutionResultID(c.IncorporatedResult.Result.ID()))

	// this approval has to be cached since execution result is not known yet
	err := c.core.ProcessApproval(approval)
	require.NoError(c.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = c.core.ProcessIncorporatedResult(c.IncorporatedResult)
	require.Error(c.T(), err)
}
