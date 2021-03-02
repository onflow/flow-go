package assigner

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockassigner "github.com/onflow/flow-go/engine/verification/assigner/mock"
	"github.com/onflow/flow-go/engine/verification/test"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignerEngineTestSuite encapsulates data structures for running unittests on assigner engine.
type AssignerEngineTestSuite struct {
	// modules
	me               *module.Local
	state            *protocol.State
	snapshot         *protocol.Snapshot
	metrics          *module.VerificationMetrics
	tracer           *trace.NoopTracer
	assigner         *module.ChunkAssigner
	chunksQueue      *storage.ChunksQueue
	newChunkListener *module.NewJobListener
	notifier         *module.ProcessingNotifier
	indexer          *mockassigner.Indexer

	// identities
	verIdentity *flow.Identity // verification node
}

// mockChunkAssigner mocks the chunk assigner of this test suite to assign the chunks based on the input assignment.
// It returns number of chunks assigned to verification node of this test suite.
func (s *AssignerEngineTestSuite) mockChunkAssigner(result *flow.ExecutionResult, assignment *chunks.Assignment) int {
	s.assigner.On("Assign", result, result.BlockID).Return(assignment, nil).Once()
	assignedChunks := assignment.ByNodeID(s.myID())
	return len(assignedChunks)
}

// mockStateAtBlockID is a test helper that mocks the protocol state of test suite at the given block id. This is the
// underlying protocol state of the verification node of the test suite.
func (s *AssignerEngineTestSuite) mockStateAtBlockID(blockID flow.Identifier) {
	s.state.On("AtBlockID", blockID).Return(s.snapshot)
	s.snapshot.On("Identity", s.verIdentity.NodeID).Return(s.verIdentity, nil)
}

// myID is a test helper that returns identifier of verification identity.
func (s *AssignerEngineTestSuite) myID() flow.Identifier {
	return s.verIdentity.NodeID
}

func WithIdentity(identity *flow.Identity) func(*AssignerEngineTestSuite) {
	return func(testSuite *AssignerEngineTestSuite) {
		testSuite.verIdentity = identity
	}
}

// SetupTest initiates the test setups prior to each test.
func SetupTest(options ...func(suite *AssignerEngineTestSuite)) *AssignerEngineTestSuite {
	s := &AssignerEngineTestSuite{
		me:               &module.Local{},
		state:            &protocol.State{},
		snapshot:         &protocol.Snapshot{},
		metrics:          &module.VerificationMetrics{},
		tracer:           trace.NewNoopTracer(),
		assigner:         &module.ChunkAssigner{},
		chunksQueue:      &storage.ChunksQueue{},
		newChunkListener: &module.NewJobListener{},
		verIdentity:      unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
		notifier:         &module.ProcessingNotifier{},
		indexer:          &mockassigner.Indexer{},
	}

	for _, apply := range options {
		apply(s)
	}
	return s
}

// createContainerBlock creates and returns a block that contains an execution receipt, with its corresponding chunks assignment based
// on the input options.
func createContainerBlock(options ...func(result *flow.ExecutionResult, assignments *chunks.Assignment)) (*flow.Block, *chunks.Assignment) {
	result, assignment := test.CreateExecutionResult(unittest.IdentifierFixture(), options...)
	receipt := &flow.ExecutionReceipt{
		ExecutorID:      unittest.IdentifierFixture(),
		ExecutionResult: *result,
	}
	// container block
	header := unittest.BlockHeaderFixture()
	block := &flow.Block{
		Header: &header,
		Payload: &flow.Payload{
			Receipts: []*flow.ExecutionReceipt{receipt},
		},
	}
	return block, assignment
}

// NewAssignerEngine returns an assigner engine for testing.
func NewAssignerEngine(s *AssignerEngineTestSuite) *Engine {

	e := New(zerolog.Logger{},
		s.metrics,
		s.tracer,
		s.me,
		s.state,
		s.assigner,
		s.chunksQueue,
		s.newChunkListener,
		s.indexer)

	e.withBlockConsumerNotifier(s.notifier)

	// mocks identity of the verification node
	s.me.On("NodeID").Return(s.verIdentity.NodeID)

	return e
}

// TestAssignerEngine runs all subtests in parallel.
func TestAssignerEngine(t *testing.T) {
	t.Parallel()
	t.Run("new block happy path", func(t *testing.T) {
		newBlockHappyPath(t)
	})
	t.Run("new block unstaked", func(t *testing.T) {
		newBlockUnstaked(t)
	})
	t.Run("new block zero chunk", func(t *testing.T) {
		newBlockNoChunk(t)
	})
	t.Run("new block no assigned chunk", func(t *testing.T) {
		newBlockNoAssignedChunk(t)
	})
	t.Run("new block multiple assignments", func(t *testing.T) {
		newBlockMultipleAssignment(t)
	})
	t.Run("chunk queue unhappy path duplicate", func(t *testing.T) {
		chunkQueueUnhappyPathDuplicate(t)
	})
}

// newBlockHappyPath evaluates that passing a new finalized block to assigner engine that contains
// a receipt with one assigned chunk, results in the assigner engine passing the assigned chunk to the
// chunks queue and notifying the job listener of the assigned chunks.
func newBlockHappyPath(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains
	// one assigned chunk to verification node.
	containerBlock, assignment := createContainerBlock(
		test.WithChunks(
			test.WithAssignee(s.myID())))
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)
	chunksNum := s.mockChunkAssigner(result, assignment)
	require.Equal(t, chunksNum, 1) // one chunk should be assigned

	// mocks processing assigned chunks
	// each assigned chunk should be stored in the chunks queue and new chunk listener should be
	// invoked for it.
	// Also, once all receipts of the block processed, engine should notify the block consumer once, that
	// it is done with processing this chunk.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).Return(true, nil).Times(chunksNum)
	s.newChunkListener.On("Check").Return().Times(chunksNum)
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends containerBlock containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue,
		s.newChunkListener,
		s.notifier,
		s.indexer)
}

// newBlockUnstaked evaluates that when verification node is unstaked at a reference block,
// it drops the corresponding execution receipts for that block without performing any chunk assignment.
// It also evaluates that the chunks queue is never called on any chunks of that receipt's result.
func newBlockUnstaked(t *testing.T) {
	// creates an assigner engine for an unstaked verification node.
	s := SetupTest(WithIdentity(
		unittest.IdentityFixture(
			unittest.WithRole(flow.RoleVerification),
			unittest.WithStake(0))))
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains
	// no assigned chunk to verification node.
	containerBlock, _ := createContainerBlock(
		test.WithChunks( // all chunks assigned to some (random) identifiers, but not this verification node
			test.WithAssignee(unittest.IdentifierFixture()),
			test.WithAssignee(unittest.IdentifierFixture()),
			test.WithAssignee(unittest.IdentifierFixture())))
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)

	// once assigner engine is done processing the block, it should notify the processing notifier.
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	// when the node is unstaked at reference block id, chunk assigner should not be called,
	// and nothing should be passed to chunks queue, and
	// job listener should not be notified.
	s.chunksQueue.AssertNotCalled(t, "StoreChunkLocator")
	s.newChunkListener.AssertNotCalled(t, "Check")
	s.assigner.AssertNotCalled(t, "Assign")

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.notifier,
		s.indexer)
}

// newBlockNoChunk evaluates passing a new finalized block to assigner engine that contains
// a receipt with no chunk in its result. Assigner engine should
// not pass any chunk to the chunks queue, and should never notify the job listener.
func newBlockNoChunk(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains no chunks.
	containerBlock, assignment := createContainerBlock()
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)
	chunksNum := s.mockChunkAssigner(result, assignment)
	require.Equal(t, chunksNum, 0) // no chunk should be assigned

	// once assigner engine is done processing the block, it should notify the processing notifier.
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.notifier,
		s.indexer)

	// when there is no chunk, nothing should be passed to chunks queue, and
	// job listener should not be notified.
	s.chunksQueue.AssertNotCalled(t, "StoreChunkLocator")
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// newBlockNoAssignedChunk evaluates passing a new finalized block to assigner engine that contains
// a receipt with no assigned chunk for the verification node in its result. Assigner engine should
// not pass any chunk to the chunks queue, and should not notify the job listener.
func newBlockNoAssignedChunk(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains 5 chunks, but
	// none of them is assigned to this verification node.
	containerBlock, assignment := createContainerBlock(
		test.WithChunks(
			test.WithAssignee(unittest.IdentifierFixture()),  // assigned to others
			test.WithAssignee(unittest.IdentifierFixture()),  // assigned to others
			test.WithAssignee(unittest.IdentifierFixture()),  // assigned to others
			test.WithAssignee(unittest.IdentifierFixture()),  // assigned to others
			test.WithAssignee(unittest.IdentifierFixture()))) // assigned to others
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)
	chunksNum := s.mockChunkAssigner(result, assignment)
	require.Equal(t, chunksNum, 0) // no chunk should be assigned

	// once assigner engine is done processing the block, it should notify the processing notifier.
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.notifier,
		s.indexer)

	// when there is no assigned chunk, nothing should be passed to chunks queue, and
	// job listener should not be notified.
	s.chunksQueue.AssertNotCalled(t, "StoreChunkLocator")
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// newBlockMultipleAssignment evaluates that passing a new finalized block to assigner engine that contains
// a receipt with multiple assigned chunk, results in the assigner engine passing all assigned chunks to the
// chunks queue and notifying the job listener of the assigned chunks.
func newBlockMultipleAssignment(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains 5 chunks, but
	// only 3 of them is assigned to this verification node.
	containerBlock, assignment := createContainerBlock(
		test.WithChunks(
			test.WithAssignee(unittest.IdentifierFixture()), // assigned to others
			test.WithAssignee(s.myID()),                     // assigned to me
			test.WithAssignee(s.myID()),                     // assigned to me
			test.WithAssignee(unittest.IdentifierFixture()), // assigned to others
			test.WithAssignee(s.myID())))                    // assigned to me
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)
	chunksNum := s.mockChunkAssigner(result, assignment)
	require.Equal(t, chunksNum, 3) // 3 chunks should be assigned

	// mocks processing assigned chunks
	// each assigned chunk should be stored in the chunks queue and new chunk listener should be
	// invoked for it.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).Return(true, nil).Times(chunksNum)
	s.newChunkListener.On("Check").Return().Times(chunksNum)

	// once assigner engine is done processing the block, it should notify the processing notifier.
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends containerBlock containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue,
		s.notifier,
		s.newChunkListener,
		s.indexer)
}

// chunkQueueUnhappyPathDuplicate evaluates that after submitting duplicate chunk to chunk queue, assigner engine does not invoke the notifier.
// This is important as without a new chunk successfully added to the chunks queue, the consumer should not be notified.
func chunkQueueUnhappyPathDuplicate(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	// creates a container block, with a single receipt, that contains a single chunk assigned
	// to verification node.
	containerBlock, assignment := createContainerBlock(
		test.WithChunks(test.WithAssignee(s.myID())))
	result := &containerBlock.Payload.Receipts[0].ExecutionResult
	s.mockStateAtBlockID(result.BlockID)
	chunksNum := s.mockChunkAssigner(result, assignment)
	require.Equal(t, chunksNum, 1)

	// mocks processing assigned chunks
	// adding new chunks to queue returns false, which means a duplicate chunk.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(false, nil).
		Times(chunksNum)

	// once assigner engine is done processing the block, it should notify the processing notifier.
	s.notifier.On("Notify", containerBlock.ID()).Return().Once()

	// mocks indexer module
	// on receiving a new finalized block, indexer indexes all its receipts
	s.indexer.On("Index", containerBlock.Payload.Receipts).Return(nil).Once()

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue,
		s.notifier,
		s.indexer)

	// job listener should not be notified as no new chunk is added.
	s.newChunkListener.AssertNotCalled(t, "Check")
}
