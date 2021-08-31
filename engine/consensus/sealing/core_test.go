package sealing

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestApprovalProcessingCore performs testing of approval processing core
// Core is responsible for delegating processing to assignment collectorTree for each separate execution result
// Core performs height based checks and decides if approval or incorporated result has to be processed at all
// or rejected as outdated or unverifiable.
// Core maintains a LRU cache of known approvals that cannot be verified at the moment/
func TestApprovalProcessingCore(t *testing.T) {
	suite.Run(t, new(ApprovalProcessingCoreTestSuite))
}

// RequiredApprovalsForSealConstructionTestingValue defines the number of approvals that are
// required to construct a seal for testing purposes. Thereby, the default production value
// can be set independently without changing test behaviour.
const RequiredApprovalsForSealConstructionTestingValue = 1

type ApprovalProcessingCoreTestSuite struct {
	approvals.BaseAssignmentCollectorTestSuite

	sealsDB *storage.Seals
	core    *Core
}

func (s *ApprovalProcessingCoreTestSuite) TearDownTest() {
	s.BaseAssignmentCollectorTestSuite.TearDownTest()
}

func (s *ApprovalProcessingCoreTestSuite) SetupTest() {
	s.BaseAssignmentCollectorTestSuite.SetupTest()

	s.sealsDB = &storage.Seals{}

	s.State.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil)).Maybe()

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	options := Config{
		EmergencySealingActive:               false,
		RequiredApprovalsForSealConstruction: uint(len(s.AuthorizedVerifiers)),
		ApprovalRequestsThreshold:            2,
	}

	var err error
	s.core, err = NewCore(unittest.Logger(), s.WorkerPool, tracer, metrics, &tracker.NoopSealingTracker{}, engine.NewUnit(), s.Headers, s.State, s.sealsDB, s.Assigner, s.SigVerifier, s.SealsPL, s.Conduit, options)
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

	err = s.core.ProcessFinalizedBlock(s.Block.ID())
	require.NoError(s.T(), err)

	err = s.core.processApproval(approval)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

// TestOnBlockFinalized_RejectOutdatedExecutionResult tests that incorporated result will be rejected as outdated
// if the block which is targeted by execution result is already sealed.
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOutdatedExecutionResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	err := s.core.ProcessFinalizedBlock(s.Block.ID())
	require.NoError(s.T(), err)

	err = s.core.processIncorporatedResult(s.IncorporatedResult)
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

	s.Blocks[blockB1.ID()] = &blockB1
	s.Blocks[blockB2.ID()] = &blockB2

	IR1 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB1.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	IR2 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB2.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	s.MarkFinalized(&blockB1)

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	// blockB1 becomes finalized
	err := s.core.ProcessFinalizedBlock(blockB1.ID())
	require.NoError(s.T(), err)

	err = s.core.processIncorporatedResult(IR1)
	require.NoError(s.T(), err)

	err = s.core.processIncorporatedResult(IR2)
	require.Error(s.T(), err)
	require.True(s.T(), engine.IsOutdatedInputError(err))
}

func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_RejectOldFinalizedBlock() {
	blockB1 := unittest.BlockHeaderWithParentFixture(&s.Block)
	blockB2 := unittest.BlockHeaderWithParentFixture(&blockB1)

	s.Blocks[blockB1.ID()] = &blockB1
	s.Blocks[blockB2.ID()] = &blockB2

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	// should only call it once
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()
	s.MarkFinalized(&blockB1)
	s.MarkFinalized(&blockB2)

	// blockB1 becomes finalized
	err := s.core.ProcessFinalizedBlock(blockB2.ID())
	require.NoError(s.T(), err)

	err = s.core.ProcessFinalizedBlock(blockB1.ID())
	require.NoError(s.T(), err)
}

// TestProcessFinalizedBlock_CollectorsCleanup tests that stale collectorTree are cleaned up for
// already sealed blocks.
func (s *ApprovalProcessingCoreTestSuite) TestProcessFinalizedBlock_CollectorsCleanup() {
	blockID := s.Block.ID()
	numResults := uint(10)
	for i := uint(0); i < numResults; i++ {
		// all results incorporated in different blocks
		incorporatedBlock := unittest.BlockHeaderWithParentFixture(&s.IncorporatedBlock)
		s.Blocks[incorporatedBlock.ID()] = &incorporatedBlock
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
	require.Equal(s.T(), uint64(numResults), s.core.collectorTree.GetSize())

	candidate := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.Blocks[candidate.ID()] = &candidate

	// candidate becomes new sealed and finalized block, it means that
	// we will need to cleanup our tree till new height, removing all outdated collectors
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&candidate))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	s.MarkFinalized(&candidate)
	err := s.core.ProcessFinalizedBlock(candidate.ID())
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(0), s.core.collectorTree.GetSize())
}

// TestProcessIncorporated_ApprovalsBeforeResult tests a scenario when first we have received approvals for unknown
// execution result and after that we discovered execution result. In this scenario we should be able
// to create a seal right after discovering execution result since all approvals should be cached.(if cache capacity is big enough)
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsBeforeResult() {
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

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

	s.SealsPL.On("Add", mock.Anything).Return(true, nil).Once()

	err := s.core.processIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	s.SealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ApprovalsAfterResult tests a scenario when first we have discovered execution result
//// and after that we started receiving approvals. In this scenario we should be able to create a seal right
//// after processing last needed approval to meet `RequiredApprovalsForSealConstruction` threshold.
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ApprovalsAfterResult() {
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	s.SealsPL.On("Add", mock.Anything).Return(true, nil).Once()

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

	s.SealsPL.AssertCalled(s.T(), "Add", mock.Anything)
}

// TestProcessIncorporated_ProcessingInvalidApproval tests that processing invalid approval when result is discovered
// is correctly handled in case of sentinel error
func (s *ApprovalProcessingCoreTestSuite) TestProcessIncorporated_ProcessingInvalidApproval() {
	// fail signature verification for first approval
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

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
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(false, fmt.Errorf("exception")).Once()

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
	s.core.config.EmergencySealingActive = true
	s.SealsPL.On("ByID", mock.Anything).Return(nil, false).Maybe()
	s.SealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			require.Equal(s.T(), s.Block.ID(), seal.Seal.BlockID)
			require.Equal(s.T(), s.IncorporatedResult.Result.ID(), seal.Seal.ResultID)
		},
	).Return(true, nil).Once()

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Times(approvals.DefaultEmergencySealingThreshold)
	s.State.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil))

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	lastFinalizedBlock := &s.IncorporatedBlock
	s.MarkFinalized(lastFinalizedBlock)
	for i := 0; i < approvals.DefaultEmergencySealingThreshold; i++ {
		finalizedBlock := unittest.BlockHeaderWithParentFixture(lastFinalizedBlock)
		s.Blocks[finalizedBlock.ID()] = &finalizedBlock
		s.MarkFinalized(&finalizedBlock)
		err := s.core.ProcessFinalizedBlock(finalizedBlock.ID())
		require.NoError(s.T(), err)
		lastFinalizedBlock = &finalizedBlock
	}

	s.SealsPL.AssertExpectations(s.T())
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
			s.Blocks[block.ID()] = block.Header
			s.IdentitiesCache[block.ID()] = s.AuthorizedVerifiers

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
	finalized := forks[0][0].Header
	s.MarkFinalized(finalized)
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// verify will be called twice for every approval in first fork
	s.SigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Times(len(forkResults[0]) * 2)

	// try submitting approvals for each result
	for _, results := range forkResults {
		for _, result := range results {
			executedBlockID := result.BlockID
			resultID := result.ID()

			approval := unittest.ResultApprovalFixture(unittest.WithChunk(0),
				unittest.WithApproverID(s.VerID),
				unittest.WithBlockID(executedBlockID),
				unittest.WithExecutionResultID(resultID))

			err := s.core.processApproval(approval)
			require.NoError(s.T(), err)
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
			s.Blocks[block.ID()] = block.Header
			s.IdentitiesCache[block.ID()] = s.AuthorizedVerifiers
		}
	}

	finalized := forks[1][0].Header

	s.MarkFinalized(finalized)
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	// finalize block B
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// create incorporated result for each block in main fork
	for forkIndex, fork := range forks {
		previousResult := s.IncorporatedResult.Result
		for blockIndex, block := range fork {
			result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*previousResult))
			result.BlockID = block.Header.ParentID
			result.Chunks = s.Chunks
			previousResult = result

			// incorporate in fork
			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithIncorporatedBlockID(block.ID()),
				unittest.IncorporatedResult.WithResult(result))
			err := s.core.processIncorporatedResult(IR)
			collector := s.core.collectorTree.GetCollector(result.ID())
			if forkIndex > 0 {
				require.NoError(s.T(), err)
				require.Equal(s.T(), approvals.VerifyingApprovals, collector.ProcessingStatus())
			} else {
				if blockIndex == 0 {
					require.Error(s.T(), err)
					require.True(s.T(), engine.IsOutdatedInputError(err))
				} else {
					require.Equal(s.T(), approvals.CachingApprovals, collector.ProcessingStatus())
				}
			}
		}
	}
}

// TestOnBlockFinalized_ExtendingSealedResult tests if assignment collector tree accepts collector which extends sealed result
func (s *ApprovalProcessingCoreTestSuite) TestOnBlockFinalized_ExtendingSealedResult() {
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	unsealedBlock := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.Blocks[unsealedBlock.ID()] = &unsealedBlock
	s.IdentitiesCache[unsealedBlock.ID()] = s.AuthorizedVerifiers
	result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*s.IncorporatedResult.Result))
	result.BlockID = unsealedBlock.ID()

	s.MarkFinalized(&unsealedBlock)
	err := s.core.ProcessFinalizedBlock(unsealedBlock.ID())
	require.NoError(s.T(), err)

	incorporatedBlock := unittest.BlockHeaderWithParentFixture(&unsealedBlock)
	s.Blocks[incorporatedBlock.ID()] = &incorporatedBlock
	s.IdentitiesCache[incorporatedBlock.ID()] = s.AuthorizedVerifiers
	IR := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(incorporatedBlock.ID()),
		unittest.IncorporatedResult.WithResult(result))
	err = s.core.processIncorporatedResult(IR)
	require.NoError(s.T(), err)

	s.sealsDB.AssertExpectations(s.T())
}

// TestRequestPendingApprovals checks that requests are sent only for chunks
// that have not collected enough approvals yet, and are sent only to the
// verifiers assigned to those chunks. It also checks that the threshold and
// rate limiting is respected.
func (s *ApprovalProcessingCoreTestSuite) TestRequestPendingApprovals() {
	s.core.requestTracker = approvals.NewRequestTracker(s.core.headers, 1, 3)
	s.SealsPL.On("ByID", mock.Anything).Return(nil, false)

	// n is the total number of blocks and incorporated-results we add to the
	// chain and mempool
	n := 100

	// create blocks
	unsealedFinalizedBlocks := make([]flow.Block, 0, n)
	parentBlock := &s.ParentBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock)
		s.Blocks[block.ID()] = block.Header
		s.IdentitiesCache[block.ID()] = s.AuthorizedVerifiers
		unsealedFinalizedBlocks = append(unsealedFinalizedBlocks, block)
		parentBlock = block.Header
	}

	// progress latest sealed and latest finalized:
	//s.LatestSealedBlock = unsealedFinalizedBlocks[0]
	//s.LatestFinalizedBlock = &unsealedFinalizedBlocks[n-1]

	// add an unfinalized block; it shouldn't require an approval request
	unfinalizedBlock := unittest.BlockWithParentFixture(parentBlock)
	s.Blocks[unfinalizedBlock.ID()] = unfinalizedBlock.Header

	// we will assume that all chunks are assigned to the same two verifiers.
	verifiers := make([]flow.Identifier, 0)
	for nodeID := range s.AuthorizedVerifiers {
		if len(verifiers) > 2 {
			break
		}
		verifiers = append(verifiers, nodeID)
	}

	// the sealing Core requires approvals from both verifiers for each chunk
	s.core.config.RequiredApprovalsForSealConstruction = 2

	// populate the incorporated-results tree with:
	// - 50 that have collected two signatures per chunk
	// - 25 that have collected only one signature
	// - 25 that have collected no signatures
	//
	//
	//     sealed          unsealed/finalized
	// |              ||                        |
	// 1 <- 2 <- .. <- s <- s+1 <- .. <- n-t <- n
	//                 |                  |
	//                    expected reqs
	prevResult := s.IncorporatedResult.Result
	resultIDs := make([]flow.Identifier, 0, n)
	chunkCount := 2
	for i := 0; i < n-1; i++ {

		// Create an incorporated result for unsealedFinalizedBlocks[i].
		// By default the result will contain 17 chunks.
		ir := unittest.IncorporatedResult.Fixture(
			unittest.IncorporatedResult.WithResult(
				unittest.ExecutionResultFixture(
					unittest.WithBlock(&unsealedFinalizedBlocks[i]),
					unittest.WithPreviousResult(*prevResult),
					unittest.WithChunks(uint(chunkCount)),
				),
			),
			unittest.IncorporatedResult.WithIncorporatedBlockID(
				unsealedFinalizedBlocks[i+1].ID(),
			),
		)

		prevResult = ir.Result

		s.ChunksAssignment = chunks.NewAssignment()

		for _, chunk := range ir.Result.Chunks {
			// assign the verifier to this chunk
			s.ChunksAssignment.Add(chunk, verifiers)
		}

		err := s.core.processIncorporatedResult(ir)
		require.NoError(s.T(), err)

		resultIDs = append(resultIDs, ir.Result.ID())
	}

	// sealed block doesn't change
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil)

	s.State.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil))

	// start delivering finalization events
	lastProcessedIndex := 0
	for ; lastProcessedIndex < int(s.core.config.ApprovalRequestsThreshold); lastProcessedIndex++ {
		finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
		s.MarkFinalized(finalized)
		err := s.core.ProcessFinalizedBlock(finalized.ID())
		require.NoError(s.T(), err)
	}

	require.Empty(s.T(), s.core.requestTracker.GetAllIds())

	// process two more blocks, this will trigger requesting approvals for lastSealed + 1 height
	// but they will be in blackout period
	for i := 0; i < 2; i++ {
		finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
		s.MarkFinalized(finalized)
		err := s.core.ProcessFinalizedBlock(finalized.ID())
		require.NoError(s.T(), err)
		lastProcessedIndex += 1
	}

	require.ElementsMatch(s.T(), s.core.requestTracker.GetAllIds(), resultIDs[:1])

	// wait for the max blackout period to elapse
	time.Sleep(3 * time.Second)

	// our setup is for 5 verification nodes
	s.Conduit.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(chunkCount)

	// process next block
	finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
	s.MarkFinalized(finalized)
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// now 2 results should be pending
	require.ElementsMatch(s.T(), s.core.requestTracker.GetAllIds(), resultIDs[:2])

	s.Conduit.AssertExpectations(s.T())
}

// TestRepopulateAssignmentCollectorTree tests that the
// collectors tree will contain execution results and assignment collectors will be created.
// P <- A[ER{P}] <- B[ER{A}] <- C[ER{B}] <- D[ER{C}] <- E[ER{D}]
//         |     <- F[ER{A}] <- G[ER{B}] <- H[ER{G}]
//      finalized
// collectors tree has to be repopulated with incorporated results from blocks [A, B, C, D, F, G]
// E, H shouldn't be considered since
func (s *ApprovalProcessingCoreTestSuite) TestRepopulateAssignmentCollectorTree() {
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()
	assigner := &module.ChunkAssigner{}

	// setup mocks
	payloads := &storage.Payloads{}
	expectedResults := []*flow.IncorporatedResult{s.IncorporatedResult}
	blockChildren := make([]flow.Identifier, 0)

	s.sealsDB.On("ByBlockID", s.IncorporatedBlock.ID()).Return(
		unittest.Seal.Fixture(
			unittest.Seal.WithBlock(&s.ParentBlock)), nil)

	payload := unittest.PayloadFixture(
		unittest.WithReceipts(
			unittest.ExecutionReceiptFixture(
				unittest.WithResult(s.IncorporatedResult.Result))))
	emptyPayload := flow.EmptyPayload()
	payloads.On("ByBlockID", s.Block.ID()).Return(&emptyPayload, nil)
	payloads.On("ByBlockID", s.IncorporatedBlock.ID()).Return(
		&payload, nil)

	s.IdentitiesCache[s.IncorporatedBlock.ID()] = s.AuthorizedVerifiers

	assigner.On("Assign", s.IncorporatedResult.Result, mock.Anything).Return(s.ChunksAssignment, nil)

	// two forks
	for i := 0; i < 2; i++ {
		fork := unittest.ChainFixtureFrom(i+3, &s.IncorporatedBlock)
		prevResult := s.IncorporatedResult.Result
		// create execution results for all blocks except last one, since it won't be valid by definition
		for blockIndex, block := range fork {
			blockID := block.ID()

			// create execution result for previous block in chain
			// this result will be incorporated in current block.
			result := unittest.ExecutionResultFixture(
				unittest.WithPreviousResult(*prevResult),
			)
			result.BlockID = block.Header.ParentID

			// update caches
			s.Blocks[blockID] = block.Header
			s.IdentitiesCache[blockID] = s.AuthorizedVerifiers
			blockChildren = append(blockChildren, blockID)

			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithResult(result),
				unittest.IncorporatedResult.WithIncorporatedBlockID(blockID))

			// TODO: change this test for phase 3, assigner should expect incorporated block ID, not executed
			if blockIndex < len(fork)-1 {
				assigner.On("Assign", result, blockID).Return(s.ChunksAssignment, nil)
				expectedResults = append(expectedResults, IR)
			} else {
				assigner.On("Assign", result, blockID).Return(nil, fmt.Errorf("no assignment for block without valid child"))
			}

			payload := unittest.PayloadFixture()
			payload.Results = append(payload.Results, result)
			payloads.On("ByBlockID", blockID).Return(&payload, nil)

			prevResult = result
		}
	}

	// ValidDescendants has to return all valid descendants from finalized block
	finalSnapShot := unittest.StateSnapshotForKnownBlock(&s.IncorporatedBlock, nil)
	finalSnapShot.On("ValidDescendants").Return(blockChildren, nil)
	s.State.On("Final").Return(finalSnapShot)

	core, err := NewCore(unittest.Logger(), s.WorkerPool, tracer, metrics, &tracker.NoopSealingTracker{}, engine.NewUnit(),
		s.Headers, s.State, s.sealsDB, assigner, s.SigVerifier, s.SealsPL, s.Conduit, s.core.config)
	require.NoError(s.T(), err)

	err = core.RepopulateAssignmentCollectorTree(payloads)
	require.NoError(s.T(), err)

	// check collector tree, after repopulating we should have all collectors for execution results that we have
	// traversed and they have to be processable.
	for _, incorporatedResult := range expectedResults {
		collector, err := core.collectorTree.GetOrCreateCollector(incorporatedResult.Result)
		require.NoError(s.T(), err)
		require.False(s.T(), collector.Created)
		require.Equal(s.T(), approvals.VerifyingApprovals, collector.Collector.ProcessingStatus())
	}
}
