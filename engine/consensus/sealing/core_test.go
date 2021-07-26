package sealing

import (
	"fmt"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/consensus/approvals"
	"github.com/onflow/flow-go/engine/consensus/approvals/tracker"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
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
	approvals.BaseApprovalsTestSuite

	workerPool        *workerpool.WorkerPool
	blocks            map[flow.Identifier]*flow.Header
	headers           *storage.Headers
	state             *protocol.State
	assigner          *module.ChunkAssigner
	sealsPL           *mempool.IncorporatedResultSeals
	sealsDB           *storage.Seals
	sigVerifier       *module.Verifier
	conduit           *mocknetwork.Conduit
	finalizedAtHeight map[uint64]*flow.Header
	identitiesCache   map[flow.Identifier]map[flow.Identifier]*flow.Identity // helper map to store identities for given block
	core              *Core
}

func (s *ApprovalProcessingCoreTestSuite) TearDownTest() {
	// Without this line we are risking running into weird situations where one test has finished but there are active workers
	// that are executing some work on the shared pool. Need to ensure that all pending work has been executed before
	// starting next test.
	s.workerPool.StopWait()
}

func (s *ApprovalProcessingCoreTestSuite) SetupTest() {
	s.BaseApprovalsTestSuite.SetupTest()

	s.workerPool = workerpool.New(defaultAssignmentCollectorsWorkerPoolCapacity)
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

	s.finalizedAtHeight = make(map[uint64]*flow.Header)
	s.finalizedAtHeight[s.ParentBlock.Height] = &s.ParentBlock
	s.finalizedAtHeight[s.Block.Height] = &s.Block

	s.assigner.On("Assign", mock.Anything, mock.Anything).Return(s.ChunksAssignment, nil)

	s.headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return s.blocks[blockID]
		}, func(blockID flow.Identifier) error {
			_, found := s.blocks[blockID]
			if found {
				return nil
			} else {
				return realstorage.ErrNotFound
			}
		})
	s.headers.On("ByHeight", mock.Anything).Return(
		func(height uint64) *flow.Header {
			if block, found := s.finalizedAtHeight[height]; found {
				return block
			} else {
				return nil
			}
		},
		func(height uint64) error {
			_, found := s.finalizedAtHeight[height]
			if !found {
				return realstorage.ErrNotFound
			}
			return nil
		},
	)

	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil)).Once()

	s.state.On("AtHeight", mock.Anything).Return(
		func(height uint64) realproto.Snapshot {
			if block, found := s.finalizedAtHeight[height]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[block.ID()])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)

	s.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			if block, found := s.blocks[blockID]; found {
				return unittest.StateSnapshotForKnownBlock(block, s.identitiesCache[blockID])
			} else {
				return unittest.StateSnapshotForUnknownBlock()
			}
		},
	)

	s.sealsPL.On("Size").Return(uint(0)).Maybe()                       // for metrics
	s.sealsPL.On("PruneUpToHeight", mock.Anything).Return(nil).Maybe() // noop on pruning

	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	options := Config{
		EmergencySealingActive:               false,
		RequiredApprovalsForSealConstruction: uint(len(s.AuthorizedVerifiers)),
		ApprovalRequestsThreshold:            2,
	}

	var err error
	s.core, err = NewCore(unittest.Logger(), s.workerPool, tracer, metrics, &tracker.NoopSealingTracker{}, engine.NewUnit(), s.headers, s.state, s.sealsDB, s.assigner, s.sigVerifier, s.sealsPL, s.conduit, options)
	require.NoError(s.T(), err)
}

func (s *ApprovalProcessingCoreTestSuite) markFinalized(block *flow.Header) {
	s.finalizedAtHeight[block.Height] = block
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

	s.blocks[blockB1.ID()] = &blockB1
	s.blocks[blockB2.ID()] = &blockB2

	IR1 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB1.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	IR2 := unittest.IncorporatedResult.Fixture(
		unittest.IncorporatedResult.WithIncorporatedBlockID(blockB2.ID()),
		unittest.IncorporatedResult.WithResult(s.IncorporatedResult.Result))

	s.markFinalized(&blockB1)

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

	s.blocks[blockB1.ID()] = &blockB1
	s.blocks[blockB2.ID()] = &blockB2

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.Block))
	// should only call it once
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()
	s.markFinalized(&blockB1)
	s.markFinalized(&blockB2)

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
	require.Equal(s.T(), uint64(numResults), s.core.collectorTree.GetSize())

	candidate := unittest.BlockHeaderWithParentFixture(&s.Block)
	s.blocks[candidate.ID()] = &candidate

	// candidate becomes new sealed and finalized block, it means that
	// we will need to cleanup our tree till new height, removing all outdated collectors
	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&candidate))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Once()

	s.markFinalized(&candidate)
	err := s.core.ProcessFinalizedBlock(candidate.ID())
	require.NoError(s.T(), err)
	require.Equal(s.T(), uint64(0), s.core.collectorTree.GetSize())
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
//// after processing last needed approval to meet `RequiredApprovalsForSealConstruction` threshold.
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
	s.core.config.EmergencySealingActive = true
	s.sealsPL.On("ByID", mock.Anything).Return(nil, false).Maybe()
	s.sealsPL.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			seal := args.Get(0).(*flow.IncorporatedResultSeal)
			require.Equal(s.T(), s.Block.ID(), seal.Seal.BlockID)
			require.Equal(s.T(), s.IncorporatedResult.Result.ID(), seal.Seal.ResultID)
		},
	).Return(true, nil).Once()

	seal := unittest.Seal.Fixture(unittest.Seal.WithBlock(&s.ParentBlock))
	s.sealsDB.On("ByBlockID", mock.Anything).Return(seal, nil).Times(approvals.DefaultEmergencySealingThreshold)
	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil))

	err := s.core.ProcessIncorporatedResult(s.IncorporatedResult)
	require.NoError(s.T(), err)

	lastFinalizedBlock := &s.IncorporatedBlock
	s.markFinalized(lastFinalizedBlock)
	for i := 0; i < approvals.DefaultEmergencySealingThreshold; i++ {
		finalizedBlock := unittest.BlockHeaderWithParentFixture(lastFinalizedBlock)
		s.blocks[finalizedBlock.ID()] = &finalizedBlock
		s.markFinalized(&finalizedBlock)
		err := s.core.ProcessFinalizedBlock(finalizedBlock.ID())
		require.NoError(s.T(), err)
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
	finalized := forks[0][0].Header
	s.markFinalized(finalized)
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// verify will be called twice for every approval in first fork
	s.sigVerifier.On("Verify", mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Times(len(forkResults[0]) * 2)

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
			s.blocks[block.ID()] = block.Header
			s.identitiesCache[block.ID()] = s.AuthorizedVerifiers
		}
	}

	finalized := forks[1][0].Header

	s.markFinalized(finalized)
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
	s.blocks[unsealedBlock.ID()] = &unsealedBlock
	s.identitiesCache[unsealedBlock.ID()] = s.AuthorizedVerifiers
	result := unittest.ExecutionResultFixture(unittest.WithPreviousResult(*s.IncorporatedResult.Result))
	result.BlockID = unsealedBlock.ID()

	s.markFinalized(&unsealedBlock)
	err := s.core.ProcessFinalizedBlock(unsealedBlock.ID())
	require.NoError(s.T(), err)

	incorporatedBlock := unittest.BlockHeaderWithParentFixture(&unsealedBlock)
	s.blocks[incorporatedBlock.ID()] = &incorporatedBlock
	s.identitiesCache[incorporatedBlock.ID()] = s.AuthorizedVerifiers
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
	s.sealsPL.On("ByID", mock.Anything).Return(nil, false)

	// n is the total number of blocks and incorporated-results we add to the
	// chain and mempool
	n := 100

	// create blocks
	unsealedFinalizedBlocks := make([]flow.Block, 0, n)
	parentBlock := &s.ParentBlock
	for i := 0; i < n; i++ {
		block := unittest.BlockWithParentFixture(parentBlock)
		s.blocks[block.ID()] = block.Header
		s.identitiesCache[block.ID()] = s.AuthorizedVerifiers
		unsealedFinalizedBlocks = append(unsealedFinalizedBlocks, block)
		parentBlock = block.Header
	}

	// progress latest sealed and latest finalized:
	//s.LatestSealedBlock = unsealedFinalizedBlocks[0]
	//s.LatestFinalizedBlock = &unsealedFinalizedBlocks[n-1]

	// add an unfinalized block; it shouldn't require an approval request
	unfinalizedBlock := unittest.BlockWithParentFixture(parentBlock)
	s.blocks[unfinalizedBlock.ID()] = unfinalizedBlock.Header

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
	for i := 0; i < n; i++ {

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
				unsealedFinalizedBlocks[i].ID(),
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

	s.state.On("Sealed").Return(unittest.StateSnapshotForKnownBlock(&s.ParentBlock, nil))

	// start delivering finalization events
	lastProcessedIndex := 0
	for ; lastProcessedIndex < int(s.core.config.ApprovalRequestsThreshold); lastProcessedIndex++ {
		finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
		s.markFinalized(finalized)
		err := s.core.ProcessFinalizedBlock(finalized.ID())
		require.NoError(s.T(), err)
	}

	require.Empty(s.T(), s.core.requestTracker.GetAllIds())

	// process two more blocks, this will trigger requesting approvals for lastSealed + 1 height
	// but they will be in blackout period
	for i := 0; i < 2; i++ {
		finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
		s.markFinalized(finalized)
		err := s.core.ProcessFinalizedBlock(finalized.ID())
		require.NoError(s.T(), err)
		lastProcessedIndex += 1
	}

	require.ElementsMatch(s.T(), s.core.requestTracker.GetAllIds(), resultIDs[:1])

	// wait for the max blackout period to elapse
	time.Sleep(3 * time.Second)

	// our setup is for 5 verification nodes
	s.conduit.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Times(chunkCount)

	// process next block
	finalized := unsealedFinalizedBlocks[lastProcessedIndex].Header
	s.markFinalized(finalized)
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// now 2 results should be pending
	require.ElementsMatch(s.T(), s.core.requestTracker.GetAllIds(), resultIDs[:2])

	s.conduit.AssertExpectations(s.T())
}

// TestRepopulateAssignmentCollectorTree tests that the
// collectors tree will contain execution results and assignment collectors will be created.
// P <- A[ER{P}] <- B[ER{A}] <- C[ER{B}] <- D[ER{C}] <- E
//         |     <- F[ER{A}] <- G[ER{B}] <- H
//      finalized
// collectors tree has to be repopulated with incorporated results from blocks [A, B, C, D, F, G]
func (s *ApprovalProcessingCoreTestSuite) TestRepopulateAssignmentCollectorTree() {
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

	s.identitiesCache[s.IncorporatedBlock.ID()] = s.AuthorizedVerifiers

	// two forks
	for i := 0; i < 2; i++ {
		fork := unittest.ChainFixtureFrom(i+3, &s.IncorporatedBlock)
		prevResult := s.IncorporatedResult.Result
		// create execution results for all blocks except last one, since it won't be valid by definition
		for _, block := range fork[:len(fork)-1] {
			blockID := block.ID()

			// create execution result for previous block in chain
			// this result will be incorporated in current block.
			result := unittest.ExecutionResultFixture(
				unittest.WithPreviousResult(*prevResult),
			)
			result.BlockID = block.Header.ParentID

			// update caches
			s.blocks[blockID] = block.Header
			s.identitiesCache[blockID] = s.AuthorizedVerifiers
			blockChildren = append(blockChildren, blockID)

			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithResult(result),
				unittest.IncorporatedResult.WithIncorporatedBlockID(blockID))
			expectedResults = append(expectedResults, IR)

			payload := unittest.PayloadFixture()
			payload.Results = append(payload.Results, result)
			payloads.On("ByBlockID", blockID).Return(&payload, nil)

			prevResult = result
		}
	}

	// ValidDescendants has to return all valid descendants from finalized block
	finalSnapShot := unittest.StateSnapshotForKnownBlock(&s.IncorporatedBlock, nil)
	finalSnapShot.On("ValidDescendants").Return(blockChildren, nil)
	s.state.On("Final").Return(finalSnapShot)

	err := s.core.RepopulateAssignmentCollectorTree(payloads)
	require.NoError(s.T(), err)

	// check collector tree, after repopulating we should have all collectors for execution results that we have
	// traversed and they have to be processable.
	for _, incorporatedResult := range expectedResults {
		collector, err := s.core.collectorTree.GetOrCreateCollector(incorporatedResult.Result)
		require.NoError(s.T(), err)
		require.False(s.T(), collector.Created)
		require.Equal(s.T(), approvals.VerifyingApprovals, collector.Collector.ProcessingStatus())
	}
}

// TestProcessFinalizedBlock_ProcessableAfterSealedParent tests scenario that finalized collector becomes processable
// after parent block gets sealed. More specifically this case:
// P <- A <- B[ER{A}] <- C[ER{B}] <- D[ER{C}]
//        <- E[ER{A}] <- F[ER{E}] <- G[ER{F}]
//               |
//           finalized
// Initially P was executed,  B is finalized and incorporates ER for A, C incorporates ER for B, D was forked from
// A but wasn't finalized, E incorporates ER for D.
// Let's take a case where we have collectors for ER incorporated in blocks B, C, D, E. Since we don't
// have a collector for A, {B, C, D, E} are not processable. Test that when A becomes sealed {B, C, D} become processable
// but E is unprocessable since D wasn't part of finalized fork.
// TODO: move this test to assignment_collector_tree_test when implemented an interface for assignment collectors.
func (s *ApprovalProcessingCoreTestSuite) TestProcessFinalizedBlock_ProcessableAfterSealedParent() {
	s.identitiesCache[s.IncorporatedBlock.ID()] = s.AuthorizedVerifiers
	// two forks
	forks := make([][]*flow.Block, 2)
	results := make([][]*flow.IncorporatedResult, 2)
	for i := 0; i < len(forks); i++ {
		fork := unittest.ChainFixtureFrom(3, &s.IncorporatedBlock)
		forks[i] = fork
		prevResult := s.IncorporatedResult.Result
		// create execution results for all blocks except last one, since it won't be valid by definition
		for _, block := range fork {
			blockID := block.ID()

			// create execution result for previous block in chain
			// this result will be incorporated in current block.
			result := unittest.ExecutionResultFixture(
				unittest.WithPreviousResult(*prevResult),
			)
			result.BlockID = block.Header.ParentID

			// update caches
			s.blocks[blockID] = block.Header
			s.identitiesCache[blockID] = s.AuthorizedVerifiers

			IR := unittest.IncorporatedResult.Fixture(
				unittest.IncorporatedResult.WithResult(result),
				unittest.IncorporatedResult.WithIncorporatedBlockID(blockID))

			results[i] = append(results[i], IR)

			err := s.core.ProcessIncorporatedResult(IR)
			require.NoError(s.T(), err)

			collector := s.core.collectorTree.GetCollector(IR.Result.ID())
			require.Equal(s.T(), approvals.CachingApprovals, collector.ProcessingStatus())

			prevResult = result
		}
	}

	finalized := forks[0][0].Header

	// A becomes sealed
	s.sealsDB.On("ByBlockID", finalized.ID()).Return(
		unittest.Seal.Fixture(
			unittest.Seal.WithBlock(&s.Block)), nil)

	s.markFinalized(&s.IncorporatedBlock)
	s.markFinalized(finalized)

	// B becomes finalized
	err := s.core.ProcessFinalizedBlock(finalized.ID())
	require.NoError(s.T(), err)

	// at this point collectors for forks[0] should be processable and for forks[1] not
	for forkIndex := range forks {
		for _, result := range results[forkIndex][1:] {
			collector := s.core.collectorTree.GetCollector(result.Result.ID())
			if forkIndex == 0 {
				require.Equal(s.T(), approvals.VerifyingApprovals, collector.ProcessingStatus())
			} else {
				require.Equal(s.T(), approvals.Orphaned, collector.ProcessingStatus())
			}
		}
	}
}
