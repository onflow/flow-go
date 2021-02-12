package assigner

import (
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/verification/utils"
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

	// fixtures
	completeER utils.CompleteExecutionResult
}

// SetupTest initiates the test setups prior to each test.
func SetupTest(chunksNum int) *AssignerEngineTestSuite {
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
		completeER:       utils.LightExecutionResultFixture(chunksNum),
	}
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
	s := SetupTest(1)
	e := NewAssignerEngine(s)

	// mocks verification node staked at the block of its execution result.
	stakedAtBlock(s)
	// assigns all chunks to verification node
	chunksNum := assignChunks(t, s, 1)

	// mocks processing assigned chunks
	// each assigned chunk should be stored in the chunks queue and new chunk lister should be
	// invoked for it.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(true, nil).
		Times(chunksNum)
	s.newChunkListener.On("Check").Return().Times(chunksNum)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(s.completeER.ContainerBlock)

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
	s := SetupTest(1)
	e := NewAssignerEngine(s)

	// mocks verification node staked at the block of its execution result.
	stakedAtBlock(s)

	// assigns no chunk to this verification node
	assignNoChunk(s)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(s.completeER.ContainerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner)

	// when there is no assigned chunk, nothing should be passed to chunks queue, and
	// job listener should not be notified.
	s.chunksQueue.AssertNotCalled(t, "StoreChunkLocator")
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// TestChunkQueue_UnhappyPath_Error evaluates that if chunk queue returns an error upon submission of a
// chunk to it, the new job listener is never invoked. This is important as without a new chunk successfully
// added to the chunks queue, the consumer should not be notified.
func TestChunkQueue_UnhappyPath_Error(t *testing.T) {
	s := SetupTest(1)
	e := NewAssignerEngine(s)

	// mocks verification node staked at the block of its execution result.
	stakedAtBlock(s)

	// assigns all chunks to this verification node
	chunksNum := assignChunks(t, s, 1)

	// mocks processing assigned chunks
	// adding new chunks to queue results in an error
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(false, fmt.Errorf("error")).
		Times(chunksNum)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(s.completeER.ContainerBlock)

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
	s := SetupTest(1)
	e := NewAssignerEngine(s)

	// mocks verification node staked at the block of its execution result.
	stakedAtBlock(s)

	// assigns all chunks to this verification node.
	chunksNum := assignChunks(t, s, 1)

	// mocks processing assigned chunks
	// adding new chunks to queue returns false, which means a duplicate chunk.
	s.chunksQueue.On("StoreChunkLocator", mock.Anything).
		Return(false, nil).
		Times(chunksNum)

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(s.completeER.ContainerBlock)

	mock.AssertExpectationsForObjects(t,
		s.metrics,
		s.assigner,
		s.chunksQueue)

	// job listener should not be notified as no new chunk is added.
	s.newChunkListener.AssertNotCalled(t, "Check")
}

// stakedAtBlock is a test helper that mocks the protocol state of test suite so that its verification identity is staked
// at the reference block of its execution result.
func stakedAtBlock(s *AssignerEngineTestSuite) {
	s.state.On("AtBlockID", s.completeER.Receipt.ExecutionResult.BlockID).Return(s.snapshot)
	s.snapshot.On("Identity", s.verIdentity.NodeID).Return(s.verIdentity, nil)
}

// assignChunks is a test helper that mocks assigner of this test suite to assign all chunks
// of the execution result of the test suite to verification identity of the test suite.
// It returns number of chunks assigned to verification node.
func assignChunks(t *testing.T, s *AssignerEngineTestSuite, chunksNum int) int {
	// number of assigned chunks should be less than or equal the number of chunks in
	// execution result.
	require.LessOrEqual(t, len(s.completeER.Receipt.ExecutionResult.Chunks), chunksNum)

	a := chunks.NewAssignment()
	chunks := s.completeER.Receipt.ExecutionResult.Chunks
	for _, chunk := range chunks {
		a.Add(chunk, flow.IdentifierList{s.verIdentity.NodeID})
	}
	s.assigner.On("Assign",
		&s.completeER.Receipt.ExecutionResult,
		s.completeER.Receipt.ExecutionResult.BlockID).Return(a, nil).Once()

	return len(chunks)
}

// assignNoChunk is a test helper that mocks assigner of this test suite to assign no chunk
// of the execution result of this test suite to verification identity of this test suite.
func assignNoChunk(s *AssignerEngineTestSuite) {
	s.assigner.On("Assign",
		&s.completeER.Receipt.ExecutionResult,
		s.completeER.Receipt.ExecutionResult.BlockID).Return(chunks.NewAssignment(), nil).Once()
}
