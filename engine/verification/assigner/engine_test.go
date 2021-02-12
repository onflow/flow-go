package assigner

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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
	headerStorage    *storage.Headers
	assigner         *module.ChunkAssigner
	chunksQueue      *storage.ChunksQueue
	newChunkListener *module.NewJobListener

	// identities
	verIdentity *flow.Identity // verification node
}

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

// SetupTest initiates the test setups prior to each test.
func SetupTest() *AssignerEngineTestSuite {
	return &AssignerEngineTestSuite{
		me:               &module.Local{},
		state:            &protocol.State{},
		snapshot:         &protocol.Snapshot{},
		metrics:          &module.VerificationMetrics{},
		tracer:           trace.NewNoopTracer(),
		headerStorage:    &storage.Headers{},
		assigner:         &module.ChunkAssigner{},
		chunksQueue:      &storage.ChunksQueue{},
		newChunkListener: &module.NewJobListener{},
		verIdentity:      unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification)),
	}
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
func NewAssignerEngine(s *AssignerEngineTestSuite, opts ...func(testSuite *AssignerEngineTestSuite)) *Engine {
	for _, apply := range opts {
		apply(s)
	}

	e := New(zerolog.Logger{},
		s.metrics,
		s.tracer,
		s.me,
		s.state,
		s.headerStorage,
		s.assigner,
		s.chunksQueue,
		s.newChunkListener)

	// mocks identity of the verification node
	s.me.On("NodeID").Return(s.verIdentity.NodeID)

	return e
}

func WithIdentity(identity *flow.Identity) func(*AssignerEngineTestSuite) {
	return func(testSuite *AssignerEngineTestSuite) {
		testSuite.verIdentity = identity
	}
}

// TestNewBlock_HappyPath evaluates that passing a new finalized block to assigner engine that contains
// a receipt results in the assigner engine passing all assigned chunks in the result of the receipt to the
// chunks queue and notifying the job listener of the assigned chunks.
func TestNewBlock_HappyPath(t *testing.T) {
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
	require.Equal(t, chunksNum, 1)

	// mocks processing assigned chunks
	// each assigned chunk should be stored in the chunks queue and new chunk lister should be
	// invoked for it.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).Return(true, nil).Times(chunksNum)
	s.newChunkListener.On("Check").Return().Times(chunksNum)

	// sends containerBlock containing receipt to assigner engine
	e.ProcessFinalizedBlock(containerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue,
		s.newChunkListener)
}

// TestNewBlock_NoChunk evaluates passing a new finalized block to assigner engine that contains
// a receipt with no assigned chunk for the verification node in its result. Assigner engine should
// not pass any chunk to the chunks queue, and should not notify the job listener.
func TestNewBlock_NoChunk(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	block, assignment := createContainerBlock()
	s.mockChunkAssigner(&block.Payload.Receipts[0].ExecutionResult, assignment)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(block)

	mock.AssertExpectationsForObjects(t, s.metrics, s.assigner)

	// when there is no chunk, nothing should be passed to chunks queue, and
	// job listener should not be notified.
	s.chunksQueue.AssertNotCalled(t, "StoreChunkLocator")
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// TestChunkQueue_UnhappyPath_Error evaluates that if chunk queue returns an error upon submission of a
// chunk to it, the new job listener is never invoked. This is important as without a new chunk successfully
// added to the chunks queue, the consumer should not be notified.
func TestChunkQueue_UnhappyPath_Error(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	block, assignment := createContainerBlock(
		test.WithChunks(
			test.WithAssignee(s.myID())))
	s.mockStateAtBlockID(block.ID())
	chunksNum := s.mockChunkAssigner(&block.Payload.Receipts[0].ExecutionResult, assignment)
	require.Equal(t, chunksNum, 1)

	// mocks processing assigned chunks
	// adding new chunks to queue results in an error
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(false, fmt.Errorf("error")).
		Times(chunksNum)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(block)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue)

	// job listener should not be notified as no new chunk is added.
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// TestChunkQueue_UnhappyPath_Duplicate evaluates that after submitting duplicate chunk to chunk queue, assigner engine does not invoke the notifier.
// This is important as without a new chunk successfully added to the chunks queue, the consumer should not be notified.
func TestChunkQueue_UnhappyPath_Duplicate(t *testing.T) {
	s := SetupTest()
	e := NewAssignerEngine(s)

	block, assignment := createContainerBlock(
		test.WithChunks(
			test.WithAssignee(s.myID())))
	s.mockStateAtBlockID(block.ID())
	chunksNum := s.mockChunkAssigner(&block.Payload.Receipts[0].ExecutionResult, assignment)
	require.Equal(t, chunksNum, 1)

	// mocks processing assigned chunks
	// adding new chunks to queue returns false, which means a duplicate chunk.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(false, nil).
		Times(chunksNum)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(block)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue)

	// job listener should not be notified as no new chunk is added.
	s.newChunkListener.AssertNotCalled(t, "Check")
}
