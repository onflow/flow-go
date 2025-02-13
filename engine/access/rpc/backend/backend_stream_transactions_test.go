package backend

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/onflow/flow-go/module/irrecoverable"
	protocolint "github.com/onflow/flow-go/state/protocol"

	"github.com/onflow/flow-go/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"

	accessapi "github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/index"
	access "github.com/onflow/flow-go/engine/access/mock"
	backendmock "github.com/onflow/flow-go/engine/access/rpc/backend/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/access/subscription"
	subscriptionmock "github.com/onflow/flow-go/engine/access/subscription/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/metrics"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// TransactionStatusSuite represents a suite for testing transaction status-related functionality in the Flow blockchain.
type TransactionStatusSuite struct {
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
	communicator      *backendmock.Communicator
	blockTracker      *subscriptionmock.BlockTracker
	reporter          *syncmock.IndexReporter
	indexReporter     *index.Reporter

	chainID flow.ChainID

	broadcaster    *engine.Broadcaster
	rootBlock      flow.Block
	sealedBlock    *flow.Block
	finalizedBlock *flow.Block

	blockMap map[uint64]*flow.Block

	backend *Backend

	db                  *badger.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
}

func TestTransactionStatusSuite(t *testing.T) {
	suite.Run(t, new(TransactionStatusSuite))
}

// SetupTest initializes the test dependencies, configurations, and mock objects for TransactionStatusSuite tests.
func (s *TransactionStatusSuite) SetupTest() {
	s.log = zerolog.New(zerolog.NewConsoleWriter())
	s.state = protocol.NewState(s.T())
	s.sealedSnapshot = protocol.NewSnapshot(s.T())
	s.finalSnapshot = protocol.NewSnapshot(s.T())
	s.tempSnapshot = &protocol.Snapshot{}
	s.db, s.dbDir = unittest.TempBadgerDB(s.T())

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
	s.communicator = backendmock.NewCommunicator(s.T())
	s.broadcaster = engine.NewBroadcaster()
	s.blockTracker = subscriptionmock.NewBlockTracker(s.T())
	s.reporter = syncmock.NewIndexReporter(s.T())
	s.indexReporter = index.NewReporter()
	err := s.indexReporter.Initialize(s.reporter)
	require.NoError(s.T(), err)

	s.initializeBackend()
}

// TearDownTest cleans up the db
func (s *TransactionStatusSuite) TearDownTest() {
	err := os.RemoveAll(s.dbDir)
	s.Require().NoError(err)
}

// initializeBackend sets up and initializes the backend with required dependencies, mocks, and configurations for testing.
func (s *TransactionStatusSuite) initializeBackend() {
	s.transactions.On("Store", mock.Anything).Return(nil).Maybe()

	s.execClient.On("GetTransactionResult", mock.Anything, mock.Anything).Return(nil, status.Error(codes.NotFound, "not found")).Maybe()
	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mocks.MockCloser{}, nil).Maybe()

	s.colClient.On(
		"SendTransaction",
		mock.Anything,
		mock.Anything,
	).Return(&accessproto.SendTransactionResponse{}, nil).Maybe()

	// generate blockCount consecutive blocks with associated seal, result and execution data
	s.rootBlock = unittest.BlockFixture()

	params := protocol.NewParams(s.T())
	params.On("FinalizedRoot").Return(s.rootBlock.Header).Maybe()
	s.state.On("Params").Return(params).Maybe()

	var receipts flow.ExecutionReceiptList
	executionNodes := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	receipts = unittest.ReceiptsForBlockFixture(&s.rootBlock, executionNodes.NodeIDs())
	s.receipts.On("ByBlockID", mock.AnythingOfType("flow.Identifier")).Return(receipts, nil).Maybe()
	s.finalSnapshot.On("Identities", mock.Anything).Return(executionNodes, nil).Maybe()

	progress, err := store.NewConsumerProgress(badgerimpl.ToDB(s.db), module.ConsumeProgressLastFullBlockHeight).Initialize(s.rootBlock.Header.Height)
	require.NoError(s.T(), err)
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	require.NoError(s.T(), err)

	s.sealedBlock = &s.rootBlock
	s.finalizedBlock = unittest.BlockWithParentFixture(s.sealedBlock.Header)
	s.blockMap = map[uint64]*flow.Block{
		s.sealedBlock.Header.Height:    s.sealedBlock,
		s.finalizedBlock.Header.Height: s.finalizedBlock,
	}

	backendParams := s.backendParams()
	s.backend, err = New(backendParams)
	require.NoError(s.T(), err)
}

// backendParams returns the Params configuration for the backend.
func (s *TransactionStatusSuite) backendParams() Params {
	return Params{
		State:                s.state,
		Blocks:               s.blocks,
		Headers:              s.headers,
		Collections:          s.collections,
		Transactions:         s.transactions,
		ExecutionReceipts:    s.receipts,
		ExecutionResults:     s.results,
		ChainID:              s.chainID,
		CollectionRPC:        s.colClient,
		MaxHeightRange:       DefaultMaxHeightRange,
		SnapshotHistoryLimit: DefaultSnapshotHistoryLimit,
		Communicator:         NewNodeCommunicator(false),
		AccessMetrics:        metrics.NewNoopCollector(),
		Log:                  s.log,
		BlockTracker:         s.blockTracker,
		SubscriptionHandler: subscription.NewSubscriptionHandler(
			s.log,
			s.broadcaster,
			subscription.DefaultSendTimeout,
			subscription.DefaultResponseLimit,
			subscription.DefaultSendBufferSize,
		),
		TxResultsIndex:      index.NewTransactionResultsIndex(s.indexReporter, s.transactionResults),
		EventQueryMode:      IndexQueryModeLocalOnly,
		TxResultQueryMode:   IndexQueryModeLocalOnly,
		EventsIndex:         index.NewEventsIndex(s.indexReporter, s.events),
		LastFullBlockHeight: s.lastFullBlockHeight,
		ExecNodeIdentitiesProvider: commonrpc.NewExecutionNodeIdentitiesProvider(
			s.log,
			s.state,
			s.receipts,
			nil,
			nil,
		),
		ConnFactory: s.connectionFactory,
	}
}

// initializeMainMockInstructions sets up the main mock behaviors for components used in TransactionStatusSuite tests.
func (s *TransactionStatusSuite) initializeMainMockInstructions() {
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
					return block.Header
				}
			}

			return nil
		}, nil)

		return s.tempSnapshot
	}, nil).Maybe()

	s.finalSnapshot.On("Head").Return(func() *flow.Header {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader
	}, nil).Maybe()

	s.blockTracker.On("GetStartHeightFromBlockID", mock.Anything).Return(func(_ flow.Identifier) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil).Maybe()

	s.blockTracker.On("GetHighestHeight", flow.BlockStatusFinalized).Return(func(_ flow.BlockStatus) (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil).Maybe()
}

// initializeHappyCaseMockInstructions sets up mock behaviors for a happy-case scenario in transaction status testing.
func (s *TransactionStatusSuite) initializeHappyCaseMockInstructions() {
	s.initializeMainMockInstructions()

	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Header.Height, nil).Maybe()
	s.reporter.On("HighestIndexedHeight").Return(func() (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil).Maybe()

	s.sealedSnapshot.On("Head").Return(func() *flow.Header {
		return s.sealedBlock.Header
	}, nil).Maybe()
	s.state.On("Sealed").Return(s.sealedSnapshot, nil).Maybe()

	eventsForTx := unittest.EventsFixture(1, flow.EventAccountCreated)
	eventMessages := make([]*entities.Event, 1)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	s.events.On(
		"ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier"),
	).Return(eventsForTx, nil).Maybe()
}

// initializeTransaction generate sent transaction with ref block of the current finalized block
func (s *TransactionStatusSuite) initializeTransaction() flow.Transaction {
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(s.finalizedBlock.ID())
	s.transactions.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(&transaction.TransactionBody, nil).Maybe()
	return transaction
}

// addNewFinalizedBlock sets up a new finalized block using the provided parent header and options, and optionally notifies via broadcasting.
func (s *TransactionStatusSuite) addNewFinalizedBlock(parent *flow.Header, notify bool, options ...func(*flow.Block)) {
	s.finalizedBlock = unittest.BlockWithParentFixture(parent)
	for _, option := range options {
		option(s.finalizedBlock)
	}

	s.blockMap[s.finalizedBlock.Header.Height] = s.finalizedBlock

	if notify {
		s.broadcaster.Publish()
	}
}

func (s *TransactionStatusSuite) mockTransactionResult(transactionID *flow.Identifier, hasTransactionResultInStorage *bool) {
	s.transactionResults.On(
		"ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier"),
	).Return(func(blockID, txID flow.Identifier) (*flow.LightTransactionResult, error) {
		if *hasTransactionResultInStorage {
			return &flow.LightTransactionResult{
				TransactionID:   *transactionID,
				Failed:          false,
				ComputationUsed: 0,
			}, nil
		}
		return nil, storage.ErrNotFound
	})
}

func (s *TransactionStatusSuite) addBlockWithTransaction(transaction *flow.Transaction) {
	col := flow.CollectionFromTransactions([]*flow.Transaction{transaction})
	colID := col.ID()
	guarantee := col.Guarantee()
	light := col.Light()
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true, func(block *flow.Block) {
		block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
		s.collections.On("LightByID", colID).Return(&light, nil).Maybe()
		s.collections.On("LightByTransactionID", transaction.ID()).Return(&light, nil)
		s.blocks.On("ByCollectionID", colID).Return(block, nil)
	})
}

// Create a special common function to read subscription messages from the channel and check converting it to transaction info
// and check results for correctness
func (s *TransactionStatusSuite) checkNewSubscriptionMessage(sub subscription.Subscription, txId flow.Identifier, expectedTxStatuses []flow.TransactionStatus) {
	unittest.RequireReturnsBefore(s.T(), func() {
		v, ok := <-sub.Channel()
		require.True(s.T(), ok,
			"channel closed while waiting for transaction info:\n\t- txID %x\n\t- blockID: %x \n\t- err: %v",
			txId, s.finalizedBlock.ID(), sub.Err())

		txResults, ok := v.([]*accessapi.TransactionResult)
		require.True(s.T(), ok, "unexpected response type: %T", v)
		require.Len(s.T(), txResults, len(expectedTxStatuses))

		for i, expectedTxStatus := range expectedTxStatuses {
			result := txResults[i]
			assert.Equal(s.T(), txId, result.TransactionID)
			assert.Equal(s.T(), expectedTxStatus, result.Status)
		}

	}, time.Second, fmt.Sprintf("timed out waiting for transaction info:\n\t- txID: %x\n\t- blockID: %x", txId, s.finalizedBlock.ID()))
}

// checkGracefulShutdown ensures the provided subscription shuts down gracefully within a specified timeout duration.
func (s *TransactionStatusSuite) checkGracefulShutdown(sub subscription.Subscription) {
	// Ensure subscription shuts down gracefully
	unittest.RequireReturnsBefore(s.T(), func() {
		<-sub.Channel()
		assert.NoError(s.T(), sub.Err())
	}, 100*time.Millisecond, "timed out waiting for subscription to shutdown")
}

// TestSendAndSubscribeTransactionStatusHappyCase tests the functionality of the SubscribeTransactionStatusesFromStartBlockID method in the Backend.
// It covers the emulation of transaction stages from pending to sealed, and receiving status updates.
func (s *TransactionStatusSuite) TestSendAndSubscribeTransactionStatusHappyCase() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	// Generate sent transaction with ref block of the current finalized block
	transaction := s.initializeTransaction()
	txId := transaction.ID()

	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound).Once()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	// 1. Subscribe to transaction status and receive the first message with pending status
	sub := s.backend.SendAndSubscribeTransactionStatuses(ctx, &transaction.TransactionBody, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	// 2. Make transaction reference block sealed, and add a new finalized block that includes the transaction
	s.addBlockWithTransaction(&transaction)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusFinalized})

	// 3. Add one more finalized block on top of the transaction block and add execution results to storage
	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	// 5. Stop subscription
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkGracefulShutdown(sub)
}

// TestSendAndSubscribeTransactionStatusExpired tests the functionality of the SubscribeTransactionStatusesFromStartBlockID method in the Backend
// when transaction become expired
func (s *TransactionStatusSuite) TestSendAndSubscribeTransactionStatusExpired() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeMainMockInstructions()

	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Header.Height, nil).Maybe()
	s.reporter.On("HighestIndexedHeight").Return(func() (uint64, error) {
		finalizedHeader := s.finalizedBlock.Header
		return finalizedHeader.Height, nil
	}, nil).Maybe()
	s.transactionResults.On(
		"ByBlockIDTransactionID",
		mock.AnythingOfType("flow.Identifier"),
		mock.AnythingOfType("flow.Identifier"),
	).Return(nil, storage.ErrNotFound).Maybe()

	// Generate sent transaction with ref block of the current finalized block
	transaction := s.initializeTransaction()
	txId := transaction.ID()
	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound).Once()

	// Subscribe to transaction status and receive the first message with pending status
	sub := s.backend.SendAndSubscribeTransactionStatuses(ctx, &transaction.TransactionBody, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	// Generate 600 blocks without transaction included and check, that transaction still pending
	startHeight := s.finalizedBlock.Header.Height + 1
	lastHeight := startHeight + flow.DefaultTransactionExpiry

	for i := startHeight; i <= lastHeight; i++ {
		s.sealedBlock = s.finalizedBlock
		s.addNewFinalizedBlock(s.sealedBlock.Header, false)
	}

	// Generate final blocks and check transaction expired
	s.sealedBlock = s.finalizedBlock
	err := s.lastFullBlockHeight.Set(s.sealedBlock.Header.Height)
	s.Require().NoError(err)
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExpired})

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentPending verifies the subscription behavior for a transaction starting as pending.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusWithCurrentPending() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.initializeTransaction()
	txId := transaction.ID()
	s.collections.On("LightByTransactionID", txId).Return(nil, storage.ErrNotFound).Once()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending})

	s.addBlockWithTransaction(&transaction)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusFinalized})

	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentFinalized verifies the subscription behavior for a transaction starting as finalized.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusWithCurrentFinalized() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.initializeTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending, flow.TransactionStatusFinalized})

	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusExecuted})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentExecuted verifies the subscription behavior for a transaction starting as executed.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusWithCurrentExecuted() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.initializeTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	// 3. Add one more finalized block on top of the transaction block and add execution results to storage
	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)
	sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusPending, flow.TransactionStatusFinalized, flow.TransactionStatusExecuted})

	// 4. Make the transaction block sealed, and add a new finalized block
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)
	s.checkNewSubscriptionMessage(sub, txId, []flow.TransactionStatus{flow.TransactionStatusSealed})

	//// 5. Stop subscription
	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusWithCurrentSealed verifies the subscription behavior for a transaction starting as sealed.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusWithCurrentSealed() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.initializeHappyCaseMockInstructions()

	transaction := s.initializeTransaction()
	txId := transaction.ID()

	hasTransactionResultInStorage := false
	s.mockTransactionResult(&txId, &hasTransactionResultInStorage)

	s.addBlockWithTransaction(&transaction)

	// init transaction result for storage
	hasTransactionResultInStorage = true
	s.addNewFinalizedBlock(s.finalizedBlock.Header, true)

	s.sealedBlock = s.finalizedBlock
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)

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
	s.addNewFinalizedBlock(s.sealedBlock.Header, true)

	s.checkGracefulShutdown(sub)
}

// TestSubscribeTransactionStatusFailedSubscription verifies the behavior of subscription when transaction status fails.
// Ensures failure scenarios are handled correctly, such as missing sealed header, start height, or transaction by ID.
func (s *TransactionStatusSuite) TestSubscribeTransactionStatusFailedSubscription() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Generate sent transaction with ref block of the current finalized block
	transaction := unittest.TransactionFixture()
	transaction.SetReferenceBlockID(s.finalizedBlock.ID())
	txId := transaction.ID()

	s.Run("throws irrecoverable if sealed header not available", func() {
		expectedError := storage.ErrNotFound
		s.state.On("Sealed").Return(s.sealedSnapshot, nil).Once()
		s.sealedSnapshot.On("Head").Return(nil, expectedError).Once()

		signalerCtx := irrecoverable.WithSignalerContext(ctx,
			irrecoverable.NewMockSignalerContextExpectError(s.T(), ctx, expectedError))

		sub := s.backend.SubscribeTransactionStatuses(signalerCtx, txId, entities.EventEncodingVersion_CCF_V0)
		s.Assert().ErrorContains(sub.Err(), expectedError.Error())
	})

	s.Run("if could not get start height", func() {
		s.sealedSnapshot.On("Head").Return(func() *flow.Header {
			return s.sealedBlock.Header
		}, nil).Once()
		s.state.On("Sealed").Return(s.sealedSnapshot, nil).Once()
		expectedError := storage.ErrNotFound
		s.blockTracker.On("GetStartHeightFromBlockID", s.sealedBlock.ID()).Return(uint64(0), expectedError).Once()

		sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
		s.Assert().ErrorContains(sub.Err(), expectedError.Error())
	})

	s.Run("if could not get transaction by transaction ID", func() {
		s.sealedSnapshot.On("Head").Return(func() *flow.Header {
			return s.sealedBlock.Header
		}, nil).Once()
		s.state.On("Sealed").Return(s.sealedSnapshot, nil).Once()
		s.blockTracker.On("GetStartHeightFromBlockID", mock.Anything).Return(func(_ flow.Identifier) (uint64, error) {
			finalizedHeader := s.finalizedBlock.Header
			return finalizedHeader.Height, nil
		}, nil).Once()
		expectedError := storage.ErrNotFound
		s.transactions.On("ByID", txId).Return(nil, expectedError).Once()

		sub := s.backend.SubscribeTransactionStatuses(ctx, txId, entities.EventEncodingVersion_CCF_V0)
		s.Assert().ErrorContains(sub.Err(), expectedError.Error())
	})
}
