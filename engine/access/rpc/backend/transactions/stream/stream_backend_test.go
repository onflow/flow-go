package stream

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/validator"
	validatormock "github.com/onflow/flow-go/access/validator/mock"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/index"
	access "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	trackermock "github.com/onflow/flow-go/engine/access/subscription/tracker/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocolint "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

// TransactionStreamSuite represents a suite for testing transaction status-related functionality in the Flow blockchain.
type TransactionStreamSuite struct {
	suite.Suite

	state          *protocol.State
	sealedSnapshot *protocol.Snapshot
	finalSnapshot  *protocol.Snapshot
	tempSnapshot   *protocol.Snapshot
	log            zerolog.Logger

	blocks             *storagemock.Blocks
	headers            *storagemock.Headers
	collections        *storagemock.Collections
	transactions       *storagemock.Transactions
	receipts           *storagemock.ExecutionReceipts
	results            *storagemock.ExecutionResults
	transactionResults *storagemock.LightTransactionResults
	events             *storagemock.Events
	seals              *storagemock.Seals

	colClient              *access.AccessAPIClient
	execClient             *access.ExecutionAPIClient
	historicalAccessClient *access.AccessAPIClient
	archiveClient          *access.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory

	blockTracker  *trackermock.BlockTracker
	reporter      *syncmock.IndexReporter
	indexReporter *index.Reporter
	eventIndex    *index.EventsIndex
	txResultIndex *index.TransactionResultsIndex

	chainID flow.ChainID

	broadcaster    *engine.Broadcaster
	rootBlock      *flow.Block
	sealedBlock    *flow.Block
	finalizedBlock *flow.Block

	blockMap map[uint64]*flow.Block

	txStreamBackend *TransactionStream

	db                  storage.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter

	systemTx *flow.TransactionBody

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestTransactionStatusSuite(t *testing.T) {
	suite.Run(t, new(TransactionStreamSuite))
}

// SetupTest initializes the test dependencies, configurations, and mock objects for TransactionStreamSuite tests.
func (s *TransactionStreamSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = protocol.NewState(s.T())
	s.sealedSnapshot = protocol.NewSnapshot(s.T())
	s.finalSnapshot = protocol.NewSnapshot(s.T())
	s.tempSnapshot = &protocol.Snapshot{}
	pdb, dbDir := unittest.TempPebbleDB(s.T())
	s.db = pebbleimpl.ToDB(pdb)
	s.dbDir = dbDir

	s.blocks = storagemock.NewBlocks(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.transactions = storagemock.NewTransactions(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.results = storagemock.NewExecutionResults(s.T())
	s.seals = storagemock.NewSeals(s.T())
	s.colClient = access.NewAccessAPIClient(s.T())
	s.archiveClient = access.NewAccessAPIClient(s.T())
	s.execClient = access.NewExecutionAPIClient(s.T())
	s.transactionResults = storagemock.NewLightTransactionResults(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.chainID = flow.Testnet
	s.historicalAccessClient = access.NewAccessAPIClient(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.broadcaster = engine.NewBroadcaster()
	s.blockTracker = trackermock.NewBlockTracker(s.T())
	s.reporter = syncmock.NewIndexReporter(s.T())
	s.indexReporter = index.NewReporter()
	err := s.indexReporter.Initialize(s.reporter)
	require.NoError(s.T(), err)
	s.eventIndex = index.NewEventsIndex(s.indexReporter, s.events)
	s.txResultIndex = index.NewTransactionResultsIndex(s.indexReporter, s.transactionResults)

	s.systemTx, err = blueprints.SystemChunkTransaction(s.chainID.Chain())
	s.Require().NoError(err)

	s.fixedExecutionNodeIDs = nil
	s.preferredExecutionNodeIDs = nil

	s.initializeBackend()
}

// TearDownTest cleans up the db
func (s *TransactionStreamSuite) TearDownTest() {
	err := os.RemoveAll(s.dbDir)
	s.Require().NoError(err)
}

// initializeBackend sets up and initializes the txStreamBackend with required dependencies, mocks, and configurations for testing.
func (s *TransactionStreamSuite) initializeBackend() {
	s.transactions.
		On("Store", mock.Anything).
		Return(nil).
		Maybe()

	s.execClient.
		On("GetTransactionResult", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.NotFound, "not found")).
		Maybe()

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil).
		Maybe()

	s.colClient.
		On("SendTransaction", mock.Anything, mock.Anything).
		Return(&accessproto.SendTransactionResponse{}, nil).
		Maybe()

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()

	params := protocol.NewParams(s.T())
	params.On("FinalizedRoot").Return(s.rootBlock.ToHeader()).Maybe()
	s.state.On("Params").Return(params).Maybe()

	// this line causes a S1021 lint error because receipts is explicitly declared. this is required
	// to ensure the mock library handles the response type correctly
	var receipts flow.ExecutionReceiptList //nolint:gosimple
	executionNodes := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	receipts = unittest.ReceiptsForBlockFixture(s.rootBlock, executionNodes.NodeIDs())
	s.receipts.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(receipts, nil).Maybe()
	s.finalSnapshot.On("Identities", mock.Anything).Return(executionNodes, nil).Maybe()

	progress, err := store.NewConsumerProgress(s.db, module.ConsumeProgressLastFullBlockHeight).Initialize(s.rootBlock.Height)
	require.NoError(s.T(), err)
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	require.NoError(s.T(), err)

	s.sealedBlock = s.rootBlock
	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.ToHeader())
	s.blockMap = map[uint64]*flow.Block{
		s.sealedBlock.Height:    s.sealedBlock,
		s.finalizedBlock.Height: s.finalizedBlock,
	}

	txStatusDeriver := txstatus.NewTxStatusDeriver(
		s.state,
		s.lastFullBlockHeight,
	)

	nodeCommunicator := node_communicator.NewNodeCommunicator(false)

	execNodeProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.state,
		s.receipts,
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodeIDs,
	)

	errorMessageProvider := error_messages.NewTxErrorMessageProvider(
		s.log,
		nil,
		s.txResultIndex,
		s.connectionFactory,
		nodeCommunicator,
		execNodeProvider,
	)

	localTxProvider := provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventIndex,
		s.txResultIndex,
		errorMessageProvider,
		s.systemTx.ID(),
		txStatusDeriver,
	)

	execNodeTxProvider := provider.NewENTransactionProvider(
		s.log,
		s.state,
		s.collections,
		s.connectionFactory,
		nodeCommunicator,
		execNodeProvider,
		txStatusDeriver,
		s.systemTx.ID(),
		s.systemTx,
	)

	txProvider := provider.NewFailoverTransactionProvider(localTxProvider, execNodeTxProvider)

	subscriptionHandler := subscription.NewSubscriptionHandler(
		s.log,
		s.broadcaster,
		subscription.DefaultSendTimeout,
		subscription.DefaultResponseLimit,
		subscription.DefaultSendBufferSize,
	)

	validatorBlocks := validatormock.NewBlocks(s.T())
	validatorBlocks.
		On("HeaderByID", mock.Anything).
		Return(s.finalizedBlock.ToHeader(), nil).
		Maybe() // used for some tests

	validatorBlocks.
		On("FinalizedHeader", mock.Anything).
		Return(s.finalizedBlock.ToHeader(), nil).
		Maybe() // used for some tests

	txValidator, err := validator.NewTransactionValidator(
		validatorBlocks,
		s.chainID.Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{
			MaxTransactionByteSize: flow.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:  flow.DefaultMaxCollectionByteSize,
		},
		execmock.NewScriptExecutor(s.T()),
	)
	s.Require().NoError(err)

	txResCache, err := lru.New[flow.Identifier, *accessmodel.TransactionResult](10)
	s.Require().NoError(err)

	client := access.NewAccessAPIClient(s.T())
	client.
		On("SendTransaction", mock.Anything, mock.Anything).
		Return(&accessproto.SendTransactionResponse{}, nil).
		Maybe() // used for some tests

	txParams := transactions.Params{
		Log:                         s.log,
		Metrics:                     metrics.NewNoopCollector(),
		State:                       s.state,
		ChainID:                     s.chainID,
		SystemTxID:                  s.systemTx.ID(),
		SystemTx:                    s.systemTx,
		StaticCollectionRPCClient:   client,
		HistoricalAccessNodeClients: nil,
		NodeCommunicator:            nodeCommunicator,
		ConnFactory:                 s.connectionFactory,
		EnableRetries:               false,
		NodeProvider:                execNodeProvider,
		Blocks:                      s.blocks,
		Collections:                 s.collections,
		Transactions:                s.transactions,
		TxErrorMessageProvider:      errorMessageProvider,
		TxResultCache:               txResCache,
		TxProvider:                  txProvider,
		TxValidator:                 txValidator,
		TxStatusDeriver:             txStatusDeriver,
		EventsIndex:                 s.eventIndex,
		TxResultsIndex:              s.txResultIndex,
	}
	txBackend, err := transactions.NewTransactionsBackend(txParams)
	s.Require().NoError(err)

	s.txStreamBackend = NewTransactionStreamBackend(
		s.log,
		s.state,
		subscriptionHandler,
		s.blockTracker,
		txBackend.SendTransaction,
		s.blocks,
		s.collections,
		s.transactions,
		txProvider,
		txStatusDeriver,
	)
}

// initializeMainMockInstructions sets up the main mock behaviors for components used in TransactionStreamSuite tests.
func (s *TransactionStreamSuite) initializeMainMockInstructions() {
	s.transactions.On("Store", mock.Anything).Return(nil).Maybe()

	s.blocks.On("ByHeight", mock.AnythingOfType("uint64")).Return(mocks.StorageMapGetter(s.blockMap)).Maybe()
	s.blocks.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Block {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block
				}
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return nil
				}
			}
			return errors.New("block not found")
		},
	).Maybe()

	s.state.On("Final").Return(s.finalSnapshot, nil).Maybe()
	s.state.On("AtBlockID", mock.AnythingOfType("flow.Identifier")).Return(func(blockID flow.Identifier) protocolint.Snapshot {
		s.tempSnapshot.On("Head").Unset()
		s.tempSnapshot.On("Head").Return(func() *flow.Header {
			for _, block := range s.blockMap {
				if block.ID() == blockID {
					return block.ToHeader()
				}
			}

			return nil
		}, nil)

		return s.tempSnapshot
	}, nil).Maybe()

	s.finalSnapshot.On("Head").Return(func() *flow.Header {
		return s.finalizedBlock.ToHeader()
	}, nil).Maybe()

	s.blockTracker.On("GetStartHeightFromBlockID", mock.Anything).Return(func(_ flow.Identifier) (uint64, error) {
		return s.finalizedBlock.Height, nil
	}, nil).Maybe()

	s.blockTracker.On("GetHighestHeight", flow.BlockStatusFinalized).Return(func(_ flow.BlockStatus) (uint64, error) {
		return s.finalizedBlock.Height, nil
	}, nil).Maybe()
}

// initializeHappyCaseMockInstructions sets up mock behaviors for a happy-case scenario in transaction status testing.
func (s *TransactionStreamSuite) initializeHappyCaseMockInstructions() {
	s.initializeMainMockInstructions()

	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil).Maybe()
	s.reporter.On("HighestIndexedHeight").Return(func() (uint64, error) {
		return s.finalizedBlock.Height, nil
	}, nil).Maybe()

	s.sealedSnapshot.On("Head").Return(func() *flow.Header {
		return s.sealedBlock.ToHeader()
	}, nil).Maybe()
	s.state.On("Sealed").Return(s.sealedSnapshot, nil).Maybe()

	eventsCount := 1
	eventsForTx := unittest.EventsFixture(eventsCount)
	eventMessages := make([]*entities.Event, eventsCount)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	s.events.On(
		"ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier"),
	).Return(eventsForTx, nil).Maybe()
}

// createSendTransaction generate sent transaction with ref block of the current finalized block
func (s *TransactionStreamSuite) createSendTransaction() flow.Transaction {
	transaction := unittest.TransactionFixture(func(t *flow.Transaction) {
		t.ReferenceBlockID = s.finalizedBlock.ID()
	})
	s.transactions.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(&transaction.TransactionBody, nil).Maybe()
	return transaction
}

// addNewFinalizedBlock sets up a new finalized block using the provided parent header and options, and optionally notifies via broadcasting.
func (s *TransactionStreamSuite) addNewFinalizedBlock(parent *flow.Header, notify bool, options ...func(*flow.Block)) {
	s.finalizedBlock = unittest.BlockWithParentFixture(parent)
	for _, option := range options {
		option(s.finalizedBlock)
	}

	s.blockMap[s.finalizedBlock.Height] = s.finalizedBlock

	if notify {
		s.broadcaster.Publish()
	}
}

func (s *TransactionStreamSuite) mockTransactionResult(transactionID *flow.Identifier, hasTransactionResultInStorage *bool) {
	s.transactionResults.
		On("ByBlockIDTransactionID", mock.Anything, mock.Anything).
		Return(
			func(blockID, txID flow.Identifier) (*flow.LightTransactionResult, error) {
				if *hasTransactionResultInStorage {
					return &flow.LightTransactionResult{
						TransactionID:   *transactionID,
						Failed:          false,
						ComputationUsed: 0,
					}, nil
				}
				return nil, storage.ErrNotFound
			},
		)
}

func (s *TransactionStreamSuite) addBlockWithTransaction(transaction *flow.Transaction) {
	col := unittest.CollectionFromTransactions([]*flow.Transaction{transaction})
	colID := col.ID()
	guarantee := flow.CollectionGuarantee{CollectionID: colID}
	light := col.Light()
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true, func(block *flow.Block) {
		var err error
		block, err = flow.NewBlock(
			flow.UntrustedBlock{
				HeaderBody: block.HeaderBody,
				Payload:    unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)),
			},
		)
		require.NoError(s.T(), err)
		s.collections.On("LightByID", colID).Return(light, nil).Maybe()
		s.collections.On("LightByTransactionID", transaction.ID()).Return(light, nil)
		s.blocks.On("ByCollectionID", colID).Return(block, nil)
	})
}

// Create a special common function to read subscription messages from the channel and check converting it to transaction info
// and check results for correctness
func (s *TransactionStreamSuite) checkNewSubscriptionMessage(sub subscription.Subscription, txId flow.Identifier, expectedTxStatuses []flow.TransactionStatus) {
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		require.True(s.T(), ok,
			"channel closed while waiting for transaction info:\n\t- txID %x\n\t- blockID: %x \n\t- err: %v",
			txId, s.finalizedBlock.ID(), sub.Err())

		txResults, ok := v.([]*accessmodel.TransactionResult)
		require.True(s.T(), ok, "unexpected response type: %T", v)
		require.Len(s.T(), txResults, len(expectedTxStatuses))

		for i, expectedTxStatus := range expectedTxStatuses {
			result := txResults[i]
			assert.Equal(s.T(), txId, result.TransactionID)
			assert.Equal(s.T(), expectedTxStatus, result.Status)
		}

	}, 180*time.Second, fmt.Sprintf("timed out waiting for transaction info:\n\t- txID: %x\n\t- blockID: %x", txId, s.finalizedBlock.ID()))
}

// checkGracefulShutdown ensures the provided subscription shuts down gracefully within a specified timeout duration.
func (s *TransactionStreamSuite) checkGracefulShutdown(sub subscription.Subscription) {
	// Ensure subscription shuts down gracefully
	unittest.RequireReturnsBefore(s.T(), func() {
		<-sub.Channel()
		assert.NoError(s.T(), sub.Err())
	}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
}

// TestSendAndSubscribeTransactionStatusHappyCase tests the functionality of the SubscribeTransactionStatusesFromStartBlockID method in the Backend.
// It covers the emulation of transaction stages from pending to sealed, and receiving status updates.
func (s *TransactionStreamSuite) TestSendAndSubscribeTransactionStatusHappyCase() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	// Generate sent transaction with ref block of the current finalized block
	transaction := s.createSendTransaction()
	txId := transaction.ID()

	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound).Once()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	// 1. Subscribe to transaction status and receive the first message with pending status
	sub := s.txStreamBackend.SendAndSubscribeTransactionStatuses(ctx, &transaction.TransactionBody, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	// 2. Make transaction reference block sealed, and add a new finalized block that includes the transaction
	s.addBlockWithTransaction(&transaction)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusFinalized})

	// 3. Add one more finalized block on top of the transaction block and add execution results to storage
	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	// 5. Stop subscription
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkGracefulShutdown(sub)
}

// TestSendAndSubscribeTransactionStatusExpired tests the functionality of the SubscribeTransactionStatusesFromStartBlockID method in the Backend
// when transaction become expired
func (s *TransactionStreamSuite) TestSendAndSubscribeTransactionStatusExpired() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeMainMockInstructions()

	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil).Maybe()
	s.reporter.On("HighestIndexedHeight").Return(func() (uint64, error) {
		return s.finalizedBlock.Height, nil
	}, nil).Maybe()
	s.transactionResults.On(
		"ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier"),
	).Return(nil, storage.ErrNotFound).Maybe()

	// Generate sent transaction with ref block of the current finalized block
	transaction := s.createSendTransaction()
	txId := transaction.ID()
	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound)

	// Subscribe to transaction status and receive the first message with pending status
	sub := s.txStreamBackend.SendAndSubscribeTransactionStatuses(ctx, &transaction.TransactionBody, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	// Generate 600 blocks without transaction included and check, that transaction still pending
	startHeight := s.finalizedBlock.Height + 1
	lastHeight := startHeight + flow.DefaultTransactionExpiry

	for i := startHeight; i <= lastHeight; i++ {
		s.sealedBlock = s.finalizedBlock
		s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), false)
	}

	// Generate final blocks and check transaction expired
	s.sealedBlock = s.finalizedBlock
	err := s.lastFullBlockHeight.Set(s.sealedBlock.Height)
	s.Require().NoError(err)
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExpired})

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentPending verifies the subscription behavior for a transaction starting as pending.
func (s *TransactionStreamSuite) TestSubscribeTransactionStatusWithCurrentPending() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.createSendTransaction()
	txId := transaction.ID()
	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound).Once()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	sub := s.txStreamBackend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	s.addBlockWithTransaction(&transaction)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusFinalized})

	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentFinalized verifies the subscription behavior for a transaction starting as finalized.
func (s *TransactionStreamSuite) TestSubscribeTransactionStatusWithCurrentFinalized() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.createSendTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	sub := s.txStreamBackend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending, flow.TransactionStatusFinalized})

	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentExecuted verifies the subscription behavior for a transaction starting as executed.
func (s *TransactionStreamSuite) TestSubscribeTransactionStatusWithCurrentExecuted() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.createSendTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	// 3. Add one more finalized block on top of the transaction block and add execution results to storage
	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.ToHeader(), true)
	sub := s.txStreamBackend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(
		sub,
		txId,
		[]flow.TransactionStatus{
			flow.TransactionStatusPending,
			flow.TransactionStatusFinalized,
			flow.TransactionStatusExecuted,
		})

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	//// 5. Stop subscription
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentSealed verifies the subscription behavior for a transaction starting as sealed.
func (s *TransactionStreamSuite) TestSubscribeTransactionStatusWithCurrentSealed() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.createSendTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.ToHeader(), true)

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	sub := s.txStreamBackend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)

	s.checkNewSubscriptionMessage(
		sub,
		txId,
		[]flow.TransactionStatus{
			flow.TransactionStatusPending,
			flow.TransactionStatusFinalized,
			flow.TransactionStatusExecuted,
			flow.TransactionStatusSealed,
		},
	)

	// 5. Stop subscription
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.ToHeader(), true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusFailedSubscription verifies the behavior of subscription when transaction status fails.
// Ensures failure scenarios are handled correctly, such as missing sealed header, start height, or transaction by ID.
func (s *TransactionStreamSuite) TestSubscribeTransactionStatusFailedSubscription() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate sent transaction with ref block of the current finalized block
	transaction := unittest.TransactionFixture(
		func(t *flow.Transaction) {
			t.ReferenceBlockID = s.finalizedBlock.ID()
		})
	txId := transaction.ID()

	s.Run("throws irrecoverable if sealed header not available", func() {
		expectedError := storage.ErrNotFound
		s.state.On("Sealed").Return(s.sealedSnapshot, nil).Once()
		s.sealedSnapshot.On("Head").Return(nil, expectedError).Once()

		signalerCtx := irrecoverable.WithSignalerContext(ctx,
			irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, fmt.Errorf("failed to lookup sealed block: %w", expectedError)))

		sub := s.txStreamBackend.SubscribeTransactionStatuses(signalerCtx, txId, entities.EventEncodingVersion_CCF_V0)
		s.Assert().ErrorContains(sub.Err(), fmt.Errorf("failed to lookup sealed block: %w", expectedError).Error())
	})

	s.Run("if could not get start height", func() {
		s.sealedSnapshot.On("Head").Return(func() *flow.Header {
			return s.sealedBlock.ToHeader()
		}, nil).Once()
		s.state.On("Sealed").Return(s.sealedSnapshot, nil).Once()
		expectedError := storage.ErrNotFound
		s.blockTracker.On("GetStartHeightFromBlockID", s.sealedBlock.ID()).Return(uint64(0), expectedError).Once()

		sub := s.txStreamBackend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
		s.Assert().ErrorContains(sub.Err(), expectedError.Error())
	})
}
