package approvals

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/sealing"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalProcessingCore performs testing of approval processing core
// approvalProcessingCore is responsible for delegating processing to assignment collectorTree for each separate execution result
// approvalProcessingCore performs height based checks and decides if approval or incorporated result has to be processed at all
// or rejected as outdated or unverifiable.
// approvalProcessingCore maintains a LRU cache of known approvals that cannot be verified at the moment/
func TestApprovalProcessingCore(t *testing.T) {
	suite.Run(t, new(ApprovalProcessingCoreTestSuite))
}

type ApprovalProcessingCoreTestSuite struct {
	BaseApprovalsTestSuite

	blocks          map[flow.Identifier]*flow.Header
	headers         *storage.Headers
	state           *protocol.State
	assigner        *module.ChunkAssigner
	sealsPL         *mempool.IncorporatedResultSeals
	sealsDB         *storage.Seals
	sigVerifier     *module.Verifier
	conduit         *mocknetwork.Conduit
	identitiesCache map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	core            *approvalProcessingCore
}

func (s *ApprovalProcessingCoreTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.sealsPL = &mempool.IncorporatedResultSeals{}
	s.state = &protocol.State{}
	s.assigner = &module.ChunkAssigner{}
	s.sigVerifier = &module.Verifier{}
	s.conduit = &mocknetwork.Conduit{}
	s.headers = &storage.Headers{}
	s.sealsDB = &storage.Seals{}

	// setup blocks cache for protocol state
	s.blocks = make(map[flow.Identifier]*flow.Header)
	s.blocks[s.ParentBlock.ID()] = &s.ParentBlock
	s.blocks[s.Block.ID()] = &s.Block
	s.blocks[s.IncorporatedBlock.ID()] = &s.IncorporatedBlock

	// setup identities for each block
	s.identitiesCache = make(map[flow.Identifier]map[flow.Identifier]*flow.Identity)
	s.identitiesCache[s.IncorporatedResult.Result.BlockID] = s.AuthorizedVerifiers

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(s.ChunksAssignment, nil)

	s.headers.On("ByBlockID", mock.Anything).Return(func(blockID flow.Identifier) *flow.Header {
		return s.blocks[blockID]
	}, func(blockID flow.Identifier) error {
		_, found := s.blocks[blockID]
		if found {
			return nil
		} else {
			return realstorage.ErrNotFound
		}
	})

	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil)).Once()

	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if block, found := s.blocks[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[blockID])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)
	var err error
	s.core, err = NewApprovalProcessingCore(s.headers, s.state, s.sealsDB, s.assigner, s.sigVerifier, s.sealsPL, s.conduit,
		uint(len(s.AuthorizedVerifiers)), false)
	require.NoError(s.T(), err)
}

// TestOnBlockFinalized_RejectOutdatedApprovals tests that approvals will be rejected as outdated
// for block that is already sealed
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedApprovals() {
	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(s.VerID),
		unittest.WithChunk(s.Chunks[0].Index),
		unittest.WithBlockID(s.Block.ID()))
	err := s.core.processApproval(approval)
	require.NoError(s.T(), err)

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	s.core.OnFinalizedBlock(s.Block.ID())

	err = s.core.processApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectOutdatedExecutionResult tests that incorporated result will be rejected as outdated
// if the block which is targeted by execution result is already sealed.
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedExecutionResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	s.core.OnFinalizedBlock(s.Block.ID())

	err := s.core.processIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectUnverifiableEntries tests that core will reject both execution results
// and approvals for blocks that we have no information about.
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectUnverifiableEntries() {
	s.IncorporatedResult.Result.BlockID = unittest.IdentifierFixture() // replace blockID with random one
	err := s.core.processIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsUnverifiableInputError(err))

	approval := unittest.ResultApprovalFixture(unittest.WithApproverID(s.VerID),
		unittest.WithChunk(s.Chunks[0].Index))

	err = s.core.processApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsUnverifiableInputError(err))
}

// TestOnBlockFinalized_RejectOrphanIncorporatedResults tests that execution results incorporated in orphan blocks
// are rejected as outdated in next situation
// A <- B_1
// 	 <- B_2
// B_1 is finalized rendering B_2 as orphan, submitting IR[ER[A], B_1] is a success, submitting IR[ER[A], B_2] is an outdated incorporated result
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

	s.headers.On("ByHeight", blockB1.Height).Return(&blockB1, nil)
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	// blockB1 becomes finalized
	s.core.OnFinalizedBlock(blockB1.ID())

	err := s.core.processIncorporatedResult(IR1)
	require.NoError(s.T(), err)

	err = s.core.processIncorporatedResult(IR2)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnFinalizedBlock_CollectorsCleanup tests that stale collectorTree are cleaned up for
// already sealed blocks.
func (s *ApprovalProcessingCoreTestSuite) TestOnFinalizedBlock_CollectorsCleanup() {
	blockID := s.Block.ID()
	numResults := uint(10)
	for i := uint(0); i < numResults; i++ {
		// all results incorporated in different blocks
		incorporatedBlock := unittest.BlockHeaderWithParentFixture(&s.IncorporatedBlock)
		s.blocks[incorporatedBlock.ID()] = &incorporatedBlock
		// create different incorporated results for same block ID
		result := unittest.ExecutionResultFixture()
		result.BlockID = blockID
		result.PreviousResultID = s.IncorporatedResult.Result.ID()
		incorporatedResult := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(result),
			unittest.IncorporatedResult.WithIncorporatedBlockID(incorporatedBlock.ID()))
		err := s.core.processIncorporatedResult(incorporatedResult)
		require.NoError(s.T(), err)
	}
	require.Equal(s.T(), uint64(numResults), s.core.collectorTree.size)

	candidate := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.blocks[candidate.ID()] = &candidate

	// candidate becomes new sealed and finalized block, it means that
	// we will need to cleanup our tree till new height, removing all outdated collectors
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&candidate))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	s.core.OnFinalizedBlock(candidate.ID())
	require.Equal(s.T(), uint64(0), s.core.collectorTree.size)
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
			err := s.core.processApproval(approval)
			require.NoError(s.T(), err)
		}
	}

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.core.processIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.sealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ApprovalsAfterResult tests a scenario when first we have discovered execution result
//// and after that we started receiving approvals. In this scenario we should be able to create a seal right
//// after processing last needed approval to meet `requiredApprovalsForSealConstruction` threshold.
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsAfterResult() {
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.core.processIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	for _, chunk := range s.Chunks {
		for verID := range s.AuthorizedVerifiers {
			approval := unittest.ResultApprovalFixture(unittest.WithChunk(chunk.Index),
				unittest.WithApproverID(verID),
				unittest.WithBlockID(s.Block.ID()),
				unittest.WithExecutionResultID(s.IncorporatedResult.Result.ID()))
			err := s.core.processApproval(approval)
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
	err := s.core.processApproval(approval)
	require.NoError(s.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = s.core.processIncorporatedResult(s.IncorporatedResult)
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
	err := s.core.processApproval(approval)
	require.NoError(s.T(), err)

	// at this point approval has to be processed, even if it's invalid
	// if it's an expected sentinel error, it has to be handled internally
	err = s.core.processIncorporatedResult(s.IncorporatedResult)
	require.Error(s.T(), err)
}

// TestOnBlockFinalized_EmergencySealing tests that emergency sealing kicks in to resolve sealing halt
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_EmergencySealing() {
	s.core.emergencySealingActive = true
	s.sealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Times(sealing.DefaultEmergencySealingThreshold)

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	lastFinalizedBlock := &s.IncorporatedBlock
	for i := 0; i < sealing.DefaultEmergencySealingThreshold; i++ {
		finalizedBlock := unittest.BlockHeaderWithParentFixture(lastFinalizedBlock)
		s.blocks[finalizedBlock.ID()] = &finalizedBlock
		s.core.OnFinalizedBlock(finalizedBlock.ID())
		lastFinalizedBlock = &finalizedBlock
	}

	s.sealsPL.AssertExpectations(s.T())
}

// TestOnBlockFinalized_ProcessingOrphanApprovals tests that approvals for orphan forks are rejected as outdated entries without processing
// A <- B_1 <- C_1{ IER[B_1] }
//	 <- B_2 <- C_2{ IER[B_2] } <- D_2{ IER[C_2] }
// 	 <- B_3 <- C_3{ IER[B_3] } <- D_3{ IER[C_3] } <- E_3{ IER[D_3] }
// B_1 becomes finalized rendering forks starting at B_2 and B_3 as orphans
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_ProcessingOrphanApprovals() {
	forks := make([][]*flow.Block, 3)
	forkResults := make([][]*flow.ExecutionResult, len(forks))

	for forkIndex := range forks {
		forks[forkIndex] = unittest.ChainFixtureFrom(forkIndex+2, &s.ParentBlock)
		fork := forks[forkIndex]

		previousResult := s.IncorporatedResult.Result
		for blockIndex, block := range fork {
			s.blocks[block.ID()] = block.Header
			s.identitiesCache[block.ID()] = s.AuthorizedVerifiers

			// create and incorporate result for every block in fork except first one
			if blockIndex > 0 {
				// create a result
				result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*previousResult))
				result.BlockID = block.Header.ParentID
				result.Chunks = s.Chunks
				forkResults[forkIndex] = append(forkResults[forkIndex], result)
				previousResult = result

				// incorporate in fork
				IR := unittest.IncorporatedResult.Fixture(
					unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
					unittest.IncorporatedResult.WithResult(result))

				err := s.core.processIncorporatedResult(IR)
				require.NoError(s.T(), err)
			}
		}
	}

	// same block sealed
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	// block B_1 becomes finalized
	s.core.OnFinalizedBlock(forks[0][0].ID())

	// verify will be called twice for every approval in first fork
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Times(len(forkResults[0]) * 2)

	// try submitting approvals for each result
	for forkIndex, results := range forkResults {
		for _, result := range results {
			executedBlockID := result.BlockID
			resultID := result.ID()

			approval := unittest.ResultApprovalFixture(unittest.WithChunk(0),
				unittest.WithApproverID(s.VerID),
				unittest.WithBlockID(executedBlockID),
				unittest.WithExecutionResultID(resultID))

			err := s.core.processApproval(approval)

			// for first fork all results should be valid, since it's a finalized fork
			// all others forks are orphans and approvals for those should be outdated
			if forkIndex == 0 {
				require.NoError(s.T(), err)
			} else {
				require.Error(s.T(), err)
				require.True(s.T(), engine.IsOutdatedInputError(err))
			}
		}
	}
}

// TestOnBlockFinalized_ExtendingUnprocessableFork tests that extending orphan fork results in non processable collectors
//       - X <- Y <- Z
//    /
// <- A <- B <- C <- D <- E
//		   |
//       finalized
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_ExtendingUnprocessableFork() {
	forks := make([][]*flow.Block, 2)

	for forkIndex := range forks {
		forks[forkIndex] = unittest.ChainFixtureFrom(forkIndex+3, &s.Block)
		fork := forks[forkIndex]
		for _, block := range fork {
			s.blocks[block.ID()] = block.Header
			s.identitiesCache[block.ID()] = s.AuthorizedVerifiers
		}
	}

	finalized := forks[1][0].Header

	s.headers.On("ByHeight", finalized.Height).Return(finalized, nil)
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	// finalize block B
	s.core.OnFinalizedBlock(finalized.ID())

	// create incorporated result for each block in main fork
	for forkIndex, fork := range forks {
		previousResult := s.IncorporatedResult.Result
		for _, block := range fork {
			result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*previousResult))
			result.BlockID = block.Header.ParentID
			result.Chunks = s.Chunks
			previousResult = result

			// incorporate in fork
			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
				unittest.IncorporatedResult.WithResult(result))
			err := s.core.processIncorporatedResult(IR)
			if forkIndex > 0 {
				require.NoError(s.T(), err)
			} else {
				require.Error(s.T(), err)
				require.True(s.T(), engine.IsOutdatedInputError(err))
			}
		}
	}
}

// TestOnBlockFinalized_ExtendingSealedResult tests if assignment collector tree accepts collector which extends sealed result
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_ExtendingSealedResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	unsealedBlock := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.blocks[unsealedBlock.ID()] = &unsealedBlock
	s.identitiesCache[unsealedBlock.ID()] = s.AuthorizedVerifiers
	result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*s.IncorporatedResult.Result))
	result.BlockID = unsealedBlock.ID()

	s.headers.On("ByHeight", unsealedBlock.Height).Return(unsealedBlock, nil)
	s.core.OnFinalizedBlock(unsealedBlock.ID())

	incorporatedBlock := unittest.BlockHeaderWithParentFixture(&unsealedBlock)
	s.blocks[incorporatedBlock.ID()] = &incorporatedBlock
	s.identitiesCache[incorporatedBlock.ID()] = s.AuthorizedVerifiers
	IR := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(incorporatedBlock.ID()),
		unittest.IncorporatedResult.WithResult(result))
	err := s.core.processIncorporatedResult(IR)
	require.NoError(s.T(), err)

	s.sealsDB.AssertExpectations(s.T())
}
