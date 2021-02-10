package assigner

import (
	"testing"

	"github.com/rs/zerolog"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/verification/utils"
	chmodel "github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// AssignerEngineTestSuite contains the unit tests of Assigner engine.
type AssignerEngineTestSuite struct {
	suite.Suite
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

// TestFinderEngine executes all AssignerEngineTestSuite tests.
func TestFinderEngine(t *testing.T) {
	suite.Run(t, new(AssignerEngineTestSuite))
}

// SetupTest initiates the test setups prior to each test.
func (suite *AssignerEngineTestSuite) SetupTest() {
	suite.me = &module.Local{}
	suite.state = &protocol.State{}
	suite.snapshot = &protocol.Snapshot{}
	suite.metrics = &module.VerificationMetrics{}
	suite.tracer = trace.NewNoopTracer()
	suite.headerStorage = &storage.Headers{}
	suite.assigner = &module.ChunkAssigner{}
	suite.newChunkListener = &module.NewJobListener{}
	suite.chunksQueue = &storage.ChunksQueue{}

	// generates an execution result with a single collection, chunk, and transaction.
	suite.completeER = utils.LightExecutionResultFixture(1)
	suite.verIdentity = unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))
}

//
//func WithIdentity(identity *flow.Identity) func(*AssignerEngineTestSuite) {
//	return func(testSuite *AssignerEngineTestSuite) {
//		testSuite.verIdentity = identity
//	}
//}
//
// NewAssignerEngine returns an assigner engine for testing.
func (suite *AssignerEngineTestSuite) NewAssignerEngine(opts ...func(testSuite *AssignerEngineTestSuite)) *Engine {
	for _, apply := range opts {
		apply(suite)
	}

	e := New(zerolog.Logger{},
		suite.metrics,
		suite.tracer,
		suite.me,
		suite.state,
		suite.headerStorage,
		suite.assigner,
		suite.chunksQueue,
		suite.newChunkListener)

	// mocks identity of the verification node
	suite.me.On("NodeID").Return(suite.verIdentity.NodeID)

	return e
}

// TestNewBlock_HappyPath evaluates that passing a new finalized block to assigner engine that contains
// a receipt results in the assigner engine passing all assigned chunks in the result of the receipt to the
// chunks queue and notifying the job listener of the assigne d chunks.
func (suite *AssignerEngineTestSuite) TestNewBlock_HappyPath() {
	e := suite.NewAssignerEngine()

	// assigns all chunks to this verification node
	a := chmodel.NewAssignment()
	chunks := suite.completeER.Receipt.ExecutionResult.Chunks
	for _, chunk := range chunks {
		a.Add(chunk, flow.IdentifierList{suite.verIdentity.NodeID})
	}
	suite.assigner.On("Assign",
		&suite.completeER.Receipt.ExecutionResult,
		suite.completeER.Receipt.ExecutionResult.BlockID).Return(a, nil).Once()

	// mocks processing assigned chunks
	// each assigned chunk should be stored in the chunks queue and new chunk lister should be
	// invoked for it.
	// assigns all chunks to this node.
	suite.chunksQueue.On("StoreChunkLocator", testifymock.Anything).
		Return(true, nil).
		Times(len(chunks))
	suite.newChunkListener.On("Check").Return().Times(len(chunks))

	// sends block containing receipt to assigner engine
	e.ProcessFinalizedBlock(suite.completeER.ContainerBlock)

	testifymock.AssertExpectationsForObjects(suite.T(),
		suite.metrics,
		suite.assigner,
		suite.chunksQueue,
		suite.newChunkListener)
}

// assignNoChunkToMe is a test helper that no chunk of the complete execution receipt of
// this test suite to its verification node.
func (suite *AssignerEngineTestSuite) assignNoChunkToMe() {
	a := chmodel.NewAssignment()
	suite.assigner.On("Assign",
		suite.completeER.Receipt.ExecutionResult,
		suite.completeER.Receipt.ExecutionResult.BlockID).Return(a, nil).Once()
}
