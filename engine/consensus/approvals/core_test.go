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

	blocks          map[flow.Identifier]*flow.Header
	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sigVerifier     *module.Verifier
	conduit         *mocknetwork.Conduit
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	payloads        *storage.Payloads
	core            *approvalProcessingCore
}

func (s *ApprovalProcessingCoreTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.conduit = &mocknetwork.Conduit{}

	// setup blocks cache for protocol state
	s.blocks = make(map[flow.Identifier]*flow.Header)
	s.blocks[s.Block.ID()] = &s.Block
	s.blocks[s.IncorporatedBlock.ID()] = &s.IncorporatedBlock

	// setup identities for each block
	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(s.ChunksAssignment, nil)

	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if block, found := s.blocks[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[blockID])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)
	s.payloads = &storage.Payloads{}
	s.core = NewApprovalProcessingCore(s.payloads, s.state, s.assigner, s.sigVerifier, s.sealsPL, s.conduit,
		uint(len(s.AuthorizedVerifiers)), false)
}

// TestOnBlockFinalized_RejectOutdatedApprovals tests that approvals will be rejected as outdated
// for block that is already sealed
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedApprovals() {
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(s.VerID),
		unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithBlockID(s.Block.ID()))
	err := s.core.ProcessApproval(approval)
	require.NoError(s.T(), err)

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(s.Block.ID()),
		unittest.Seal.WithResult(s.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	s.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()
	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.Block, nil)).Once()

	s.core.OnFinalizedBlock(s.Block.ID())

	err = s.core.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectOutdatedExecutionResult tests that incorporated result will be rejected as outdated
// if the block which is targeted by execution result is already sealed.
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedExecutionResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(s.Block.ID()),
		unittest.Seal.WithResult(s.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	s.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()
	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.Block, nil)).Once()

	s.core.OnFinalizedBlock(s.Block.ID())

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectUnverifiableEntries tests that core will reject both execution results
// and approvals for blocks that we have no information about.
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectUnverifiableEntries() {
	s.IncorporatedResult.Result.BlockID = unittest.IdentifierFixture() // replace blockID with random one
	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsUnverifiableInputError(err))

	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(s.VerID),
		unittest.WithChunk(s.Chunks[0].Index))

	err = s.core.ProcessApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsUnverifiableInputError(err))
}

// TestOnBlockFinalized_RejectOrphanIncorporatedResults tests that execution results incorporated in orphan blocks
// are rejected as outdated in next situation
// A <- B_1
// 	 <- B_2
// B_1 is finalized rending B_2 as orphan, submitting IR[ER[A], B_1] is a success, submitting IR[ER[A], B_2] is an outdated incorporated result
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOrphanIncorporatedResults() {
	blockB1 := unittest.BlockHeaderWithParentFixture(&s.Block)
	blockB2 := unittest.BlockHeaderWithParentFixture(&s.Block)

	s.blocks[blockB1.ID()] = &blockB1
	s.blocks[blockB2.ID()] = &blockB2

	IR1 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB1.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	IR2 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB2.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	payload := unittest.PayloadFixture()

	s.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()
	s.state.On("AtHeight", blockB1.Height).Return(unittest.StateSnapshotForKnownBlock(&blockB1, nil))
	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil)).Once()

	// blockB1 becomes finalized
	s.core.OnFinalizedBlock(blockB1.ID())

	err := s.core.ProcessIncorporatedResult(IR1)
	require.NoError(s.T(), err)

	err = s.core.ProcessIncorporatedResult(IR2)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnFinalizedBlock_CollectorsCleanup tests that stale collectors are cleaned up for
// already sealed blocks.
func (s *ApprovalProcessingCoreTestSuite) TestOnFinalizedBlock_CollectorsCleanup() {
	blockID := s.Block.ID()
	numResults := 10
	for i := 0; i < numResults; i++ {
		// all results incorporated in different blocks
		incorporatedBlock := unittest.BlockHeaderWithParentFixture(&s.IncorporatedBlock)
		s.blocks[incorporatedBlock.ID()] = &incorporatedBlock
		// create different incorporated results for same block ID
		result := unittest.ExecutionResultFixture()
		result.BlockID = blockID
		incorporatedResult := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(result),
			unittest.IncorporatedResult.WithIncorporatedBlockID(incorporatedBlock.ID()))
		err := s.core.ProcessIncorporatedResult(incorporatedResult)
		require.NoError(s.T(), err)
	}

	require.Len(s.T(), s.core.collectors, numResults)

	candidate := unittest.BlockHeaderWithParentFixture(&s.Block)

	s.blocks[candidate.ID()] = &candidate

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlockID(s.Block.ID()),
		unittest.Seal.WithResult(s.IncorporatedResult.Result))
	payload := unittest.PayloadFixture(unittest.WithSeals(seal))

	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.Block, nil))
	s.payloads.On("ByBlockID", candidate.ID()).Return(&payload, nil).Once()

	s.core.OnFinalizedBlock(candidate.ID())
	require.Empty(s.T(), s.core.collectors)
}

func (s *ApprovalProcessingCoreTestSuite) TestOnFinalizedBlock_CleanupOrphanCollectors() {
	blockB1 := unittest.BlockHeaderWithParentFixture(&s.Block)
	blockB2 := unittest.BlockHeaderWithParentFixture(&s.Block)

	s.blocks[blockB1.ID()] = &blockB1
	s.blocks[blockB2.ID()] = &blockB2

	IR1 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB1.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	IR2 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB2.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	err := s.core.ProcessIncorporatedResult(IR1)
	require.NoError(s.T(), err)

	err = s.core.ProcessIncorporatedResult(IR2)
	require.NoError(s.T(), err)

	payload := unittest.PayloadFixture()
	s.payloads.On("ByBlockID", mock.Anything).Return(&payload, nil).Once()
	s.state.On("AtHeight", blockB1.Height).Return(unittest.StateSnapshotForKnownBlock(&blockB1, nil))
	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil)).Once()

	// blockB1 becomes finalized
	s.core.OnFinalizedBlock(blockB1.ID())

	resultCollector := s.core.collectors[IR1.Result.ID()]
	require.NotNil(s.T(), resultCollector)
	require.Len(s.T(), resultCollector.collectors, 1)
	require.NotNil(s.T(), resultCollector.collectors[IR1.IncorporatedBlockID])
}

// TestProcessIncorporated_ApprovalsBeforeResult tests a scenario when first we have received approvals for unknown
// execution result and after that we discovered execution result. In this scenario we should be able
// to create a seal right after discovering execution result since all approvals should be cached.(if cache capacity is big enough)
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsBeforeResult() {
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(s.Block.ID()),
				unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))
			err := s.core.ProcessApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ApprovalsAfterResult tests a scenario when first we have discovered execution result
//// and after that we started receiving approvals. In this scenario we should be able to create a seal right
//// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsAfterResult() {
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(s.Block.ID()),
				unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))
			err := s.core.ProcessApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ProcessingInvalidApproval tests that processing invalid approval when result is discovered
// is correctly handled in case of sentinel error
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ProcessingInvalidApproval() {
	// fail signature verification for first approval
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

	// generate approvals for first chunk
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithApproverID(s.VerID),
		unittest.WithBlockID(s.Block.ID()),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))

	// this approval has to be cached since execution result is not known yet
	err := s.core.ProcessApproval(approval)
	require.NoError(s.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)
}

// TestProcessIncorporated_ApprovalVerificationException tests that processing invalid approval when result is discovered
// is correctly handled in case of exception
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalVerificationException() {
	// fail signature verification with exception
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, fmt.Errorf("exception")).Once()

	// generate approvals for first chunk
	approval := unittest.ResultApprovalFixture(unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithApproverID(s.VerID),
		unittest.WithBlockID(s.Block.ID()),
		unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))

	// this approval has to be cached since execution result is not known yet
	err := s.core.ProcessApproval(approval)
	require.NoError(s.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
}
