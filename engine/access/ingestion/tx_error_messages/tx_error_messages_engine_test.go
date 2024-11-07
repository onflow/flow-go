package tx_error_messages

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotmodel "github.com/onflow/flow-go/consensus/hotstuff/model"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	bstorage "github.com/onflow/flow-go/storage/badger"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// TxErrorMessagesEngineSuite is a test suite for the transaction error messages engine.
// It sets up the necessary mocks and dependencies to test the functionality of
// handling transaction error messages.
type TxErrorMessagesEngineSuite struct {
	suite.Suite

	log   zerolog.Logger
	proto struct {
		state    *protocol.FollowerState
		snapshot *protocol.Snapshot
		params   *protocol.Params
	}
	headers         *storage.Headers
	receipts        *storage.ExecutionReceipts
	txErrorMessages *storage.TransactionResultErrorMessages

	enNodeIDs   flow.IdentityList
	execClient  *accessmock.ExecutionAPIClient
	connFactory *connectionmock.ConnectionFactory

	blockMap    map[uint64]*flow.Block
	rootBlock   flow.Block
	sealedBlock *flow.Header

	db    *badger.DB
	dbDir string

	ctx    context.Context
	cancel context.CancelFunc
}

// TestTxErrorMessagesEngine runs the test suite for the transaction error messages engine.
func TestTxErrorMessagesEngine(t *testing.T) {
	suite.Run(t, new(TxErrorMessagesEngineSuite))
}

// TearDownTest stops the engine and cleans up the db
func (s *TxErrorMessagesEngineSuite) TearDownTest() {
	s.cancel()
	err := os.RemoveAll(s.dbDir)
	s.Require().NoError(err)
}

func (s *TxErrorMessagesEngineSuite) SetupTest() {
	s.log = zerolog.New(os.Stderr)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.db, s.dbDir = unittest.TempBadgerDB(s.T())
	// mock out protocol state
	s.proto.state = protocol.NewFollowerState(s.T())
	s.proto.snapshot = protocol.NewSnapshot(s.T())
	s.proto.params = protocol.NewParams(s.T())
	s.execClient = accessmock.NewExecutionAPIClient(s.T())
	s.connFactory = connectionmock.NewConnectionFactory(s.T())
	s.headers = storage.NewHeaders(s.T())
	s.receipts = storage.NewExecutionReceipts(s.T())
	s.txErrorMessages = storage.NewTransactionResultErrorMessages(s.T())

	blockCount := 5
	s.blockMap = make(map[uint64]*flow.Block, blockCount)
	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0
	parent := s.rootBlock.Header

	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header
		s.blockMap[block.Header.Height] = block
	}

	s.sealedBlock = parent

	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(s.blockMap),
			func(block *flow.Block) *flow.Header { return block.Header },
		),
	).Maybe()

	s.proto.state.On("Params").Return(s.proto.params)

	// Mock the finalized and sealed root block header with height 0.
	s.proto.params.On("FinalizedRoot").Return(s.rootBlock.Header, nil)
	s.proto.params.On("SealedRoot").Return(s.rootBlock.Header, nil)

	s.proto.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.sealedBlock
		},
		nil,
	).Maybe()

	s.proto.state.On("Sealed").Return(s.proto.snapshot, nil)
	s.proto.state.On("Final").Return(s.proto.snapshot, nil)

	// Create identities for 1 execution nodes.
	s.enNodeIDs = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
}

// initEngine creates a new instance of the transaction error messages engine
// and waits for it to start. It initializes the engine with mocked components and state.
func (s *TxErrorMessagesEngineSuite) initEngine(ctx irrecoverable.SignalerContext) *Engine {
	processedTxErrorMessagesBlockHeight := bstorage.NewConsumerProgress(
		s.db,
		module.ConsumeProgressEngineTxErrorMessagesBlockHeight,
	)

	execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.proto.state,
		s.receipts,
		s.enNodeIDs.NodeIDs(),
		flow.IdentifierList{},
	)

	// Initialize the backend with the mocked state, blocks, headers, transactions, etc.
	backend, err := backend.New(backend.Params{
		State:                      s.proto.state,
		Headers:                    s.headers,
		ExecutionReceipts:          s.receipts,
		ConnFactory:                s.connFactory,
		MaxHeightRange:             backend.DefaultMaxHeightRange,
		Log:                        s.log,
		SnapshotHistoryLimit:       backend.DefaultSnapshotHistoryLimit,
		Communicator:               backend.NewNodeCommunicator(false),
		ScriptExecutionMode:        backend.IndexQueryModeExecutionNodesOnly,
		TxResultQueryMode:          backend.IndexQueryModeExecutionNodesOnly,
		ChainID:                    flow.Testnet,
		ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
	})
	require.NoError(s.T(), err)

	txResultErrorMessagesCore := NewTxErrorMessagesCore(
		s.log,
		backend,
		s.txErrorMessages,
		execNodeIdentitiesProvider,
	)

	eng, err := New(
		s.log,
		s.proto.state,
		s.headers,
		processedTxErrorMessagesBlockHeight,
		txResultErrorMessagesCore,
	)
	require.NoError(s.T(), err)

	eng.ComponentManager.Start(ctx)
	<-eng.Ready()

	return eng
}

// TestOnFinalizedBlockHandleTxErrorMessages tests the handling of transaction error messages
// when a new finalized block is processed. It verifies that the engine fetches transaction
// error messages from execution nodes and stores them in the database.
func (s *TxErrorMessagesEngineSuite) TestOnFinalizedBlockHandleTxErrorMessages() {
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(s.T(), s.ctx)

	block := unittest.BlockWithParentFixture(s.sealedBlock)

	s.blockMap[block.Header.Height] = block
	s.sealedBlock = block.Header

	hotstuffBlock := hotmodel.Block{
		BlockID: block.ID(),
	}

	// mock the connection factory
	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)
	s.proto.state.On("AtBlockID", mock.Anything).Return(s.proto.snapshot)

	count := 6
	wg := sync.WaitGroup{}
	wg.Add(count)

	for _, b := range s.blockMap {
		blockID := b.ID()

		// Mock the protocol snapshot to return fixed execution node IDs.
		setupReceiptsForBlock(s.receipts, b, s.enNodeIDs.NodeIDs()[0])

		// Mock the txErrorMessages storage to confirm that error messages do not exist yet.
		s.txErrorMessages.On("Exists", blockID).
			Return(false, nil).Once()

		// Create mock transaction results with a mix of failed and non-failed transactions.
		resultsByBlockID := mockTransactionResultsByBlock(5)

		// Prepare a request to fetch transaction error messages by block ID from execution nodes.
		exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
			BlockId: blockID[:],
		}

		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).Once()

		// Prepare the expected transaction error messages that should be stored.
		expectedStoreTxErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])

		// Mock the storage of the fetched error messages into the protocol database.
		s.txErrorMessages.On("Store", blockID, expectedStoreTxErrorMessages).Return(nil).
			Run(func(args mock.Arguments) {
				// Ensure the test does not complete its work faster than necessary
				wg.Done()
			}).Once()
	}

	eng := s.initEngine(irrecoverableCtx)
	// process the block through the finalized callback
	eng.OnFinalizedBlock(&hotstuffBlock)

	// Verify that all transaction error messages were processed within the timeout.
	unittest.RequireReturnsBefore(s.T(), wg.Wait, 2*time.Second, "expect to process new block before timeout")

	// Ensure all expectations were met.
	s.txErrorMessages.AssertExpectations(s.T())
	s.headers.AssertExpectations(s.T())
	s.proto.state.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}
