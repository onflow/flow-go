package transactions

import (
	"context"
	"fmt"
	"math"
	"testing"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	providermock "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider/mock"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestTransactionsBackend(t *testing.T) {
	suite.Run(t, new(Suite))
}

type Suite struct {
	suite.Suite

	g        *fixtures.GeneratorSuite
	log      zerolog.Logger
	state    *protocolmock.State
	snapshot *protocolmock.Snapshot
	params   *protocolmock.Params

	blocks                *storagemock.Blocks
	headers               *storagemock.Headers
	collections           *storagemock.Collections
	transactions          *storagemock.Transactions
	receipts              *storagemock.ExecutionReceipts
	results               *storagemock.ExecutionResults
	lightTxResults        *storagemock.LightTransactionResults
	events                *storagemock.Events
	txResultErrorMessages *storagemock.TransactionResultErrorMessages
	scheduledTransactions *storagemock.ScheduledTransactions
	txResultCache         *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]
	lastFullBlockHeight   *counters.PersistentStrictMonotonicCounter

	executionAPIClient        *accessmock.ExecutionAPIClient
	historicalAccessAPIClient *accessmock.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory

	reporter       *syncmock.IndexReporter
	indexReporter  *index.Reporter
	eventsIndex    *index.EventsIndex
	txResultsIndex *index.TransactionResultsIndex

	errorMessageProvider error_messages.Provider

	chainID                              flow.ChainID
	systemCollection                     *flow.Collection
	pendingExecutionEvents               []flow.Event
	processScheduledTransactionEventType flow.EventType
	scheduledTransactionsEnabled         bool

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func (suite *Suite) SetupTest() {
	suite.log = unittest.Logger()
	suite.chainID = flow.Testnet
	suite.g = fixtures.NewGeneratorSuite(fixtures.WithChainID(suite.chainID))

	header := unittest.BlockHeaderFixture()
	suite.snapshot = protocolmock.NewSnapshot(suite.T())
	suite.params = protocolmock.NewParams(suite.T())
	suite.params.On("FinalizedRoot").Return(header, nil).Maybe()
	suite.params.On("SporkID").Return(unittest.IdentifierFixture(), nil).Maybe()
	suite.params.On("SporkRootBlockHeight").Return(header.Height, nil).Maybe()
	suite.params.On("SealedRoot").Return(header, nil).Maybe()

	suite.state = protocolmock.NewState(suite.T())
	suite.state.On("Params").Return(suite.params).Maybe()

	suite.blocks = storagemock.NewBlocks(suite.T())
	suite.headers = storagemock.NewHeaders(suite.T())
	suite.transactions = storagemock.NewTransactions(suite.T())
	suite.collections = storagemock.NewCollections(suite.T())
	suite.receipts = storagemock.NewExecutionReceipts(suite.T())
	suite.results = storagemock.NewExecutionResults(suite.T())
	suite.txResultErrorMessages = storagemock.NewTransactionResultErrorMessages(suite.T())
	suite.executionAPIClient = accessmock.NewExecutionAPIClient(suite.T())
	suite.lightTxResults = storagemock.NewLightTransactionResults(suite.T())
	suite.events = storagemock.NewEvents(suite.T())
	suite.scheduledTransactions = storagemock.NewScheduledTransactions(suite.T())
	suite.historicalAccessAPIClient = accessmock.NewAccessAPIClient(suite.T())
	suite.connectionFactory = connectionmock.NewConnectionFactory(suite.T())

	txResCache, err := lru.New[flow.Identifier, *accessmodel.TransactionResult](10)
	suite.Require().NoError(err)
	suite.txResultCache = txResCache

	suite.reporter = syncmock.NewIndexReporter(suite.T())
	suite.indexReporter = index.NewReporter()
	err = suite.indexReporter.Initialize(suite.reporter)
	suite.Require().NoError(err)
	suite.eventsIndex = index.NewEventsIndex(suite.indexReporter, suite.events)
	suite.txResultsIndex = index.NewTransactionResultsIndex(suite.indexReporter, suite.lightTxResults)

	suite.scheduledTransactionsEnabled = true

	// this is the system collection with scheduled transactions used as block data
	suite.pendingExecutionEvents = suite.g.PendingExecutionEvents().List(2)
	systemCollections, err := systemcollection.NewVersioned(suite.chainID.Chain(), systemcollection.Default(suite.chainID))
	suite.Require().NoError(err)
	suite.systemCollection, err = systemCollections.
		ByHeight(math.MaxUint64). // use the latest version
		SystemCollection(suite.chainID.Chain(), func() (flow.EventsList, error) {
			return suite.pendingExecutionEvents, nil
		})
	suite.Require().NoError(err)
	suite.processScheduledTransactionEventType = suite.pendingExecutionEvents[0].Type

	suite.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(newMockConsumerProgress())
	suite.Require().NoError(err)

	suite.fixedExecutionNodeIDs = nil
	suite.preferredExecutionNodeIDs = nil
	suite.errorMessageProvider = nil
}

var _ storage.ConsumerProgress = (*mockConsumerProgress)(nil)

type mockConsumerProgress struct {
	counter counters.StrictMonotonicCounter
}

func newMockConsumerProgress() *mockConsumerProgress {
	return &mockConsumerProgress{counter: counters.NewMonotonicCounter(0)}
}

func (m *mockConsumerProgress) ProcessedIndex() (uint64, error) {
	return m.counter.Value(), nil
}

func (m *mockConsumerProgress) SetProcessedIndex(processed uint64) error {
	if !m.counter.Set(processed) {
		return fmt.Errorf("value must not decrease: %d", processed)
	}
	return nil
}

func (m *mockConsumerProgress) BatchSetProcessedIndex(processed uint64, batch storage.ReaderBatchWriter) error {
	return m.SetProcessedIndex(processed)
}

func (suite *Suite) defaultTransactionsParams() Params {
	nodeProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		suite.log,
		suite.state,
		suite.receipts,
		suite.preferredExecutionNodeIDs,
		suite.fixedExecutionNodeIDs,
	)

	txStatusDeriver := txstatus.NewTxStatusDeriver(
		suite.state,
		suite.lastFullBlockHeight,
	)

	validatorBlocks := validator.NewProtocolStateBlocks(suite.state, suite.indexReporter)
	txValidator, err := validator.NewTransactionValidator(
		validatorBlocks,
		suite.chainID.Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{
			Expiry:                       flow.DefaultTransactionExpiry,
			ExpiryBuffer:                 flow.DefaultTransactionExpiryBuffer,
			AllowEmptyReferenceBlockID:   false,
			AllowUnknownReferenceBlockID: false,
			CheckScriptsParse:            false,
			MaxGasLimit:                  flow.DefaultMaxTransactionGasLimit,
			MaxTransactionByteSize:       flow.DefaultMaxTransactionByteSize,
			MaxCollectionByteSize:        flow.DefaultMaxCollectionByteSize,
			CheckPayerBalanceMode:        validator.Disabled,
		},
		execmock.NewScriptExecutor(suite.T()),
	)
	suite.Require().NoError(err)

	versionedSystemCollections, err := systemcollection.NewVersioned(
		suite.chainID.Chain(),
		systemcollection.Default(suite.chainID),
	)
	suite.Require().NoError(err)

	return Params{
		Log:     suite.log,
		Metrics: metrics.NewNoopCollector(),
		State:   suite.state,
		ChainID: flow.Testnet,
		// SystemCollection:             suite.defaultSystemCollection,
		NodeCommunicator:             node_communicator.NewNodeCommunicator(false),
		ConnFactory:                  suite.connectionFactory,
		NodeProvider:                 nodeProvider,
		Blocks:                       suite.blocks,
		Collections:                  suite.collections,
		Transactions:                 suite.transactions,
		TxErrorMessageProvider:       suite.errorMessageProvider,
		ScheduledTransactions:        suite.scheduledTransactions,
		TxResultCache:                suite.txResultCache,
		TxValidator:                  txValidator,
		TxStatusDeriver:              txStatusDeriver,
		EventsIndex:                  suite.eventsIndex,
		TxResultsIndex:               suite.txResultsIndex,
		ScheduledTransactionsEnabled: suite.scheduledTransactionsEnabled,
		SystemCollections:            versionedSystemCollections,
	}
}

// TestGetTransaction_SubmittedTx tests getting a user submitted transaction by ID returns the
// correct transaction.
func (suite *Suite) TestGetTransaction_SubmittedTx() {
	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	suite.Run("submitted transaction found", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, actual)
	})

	suite.Run("submitted transaction - unexpected error", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		expectedErr := fmt.Errorf("some other error")
		suite.transactions.
			On("ByID", txID).
			Return(nil, expectedErr).
			Once()

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(actual)
	})
}

// TestGetTransaction_SystemTx tests getting a system transaction by ID returns the correct transaction.
func (suite *Suite) TestGetTransaction_SystemTx() {
	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	tx := suite.systemCollection.Transactions[0]
	txID := tx.ID()

	suite.transactions.
		On("ByID", txID).
		Return(nil, storage.ErrNotFound).
		Once()

	actual, err := txBackend.GetTransaction(context.Background(), txID)
	suite.Require().NoError(err)
	suite.Require().Equal(tx, actual)
}

// TestGetTransaction_ScheduledTx tests getting a scheduled transaction by ID returns the correct transaction.
func (suite *Suite) TestGetTransaction_ScheduledTx() {
	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	suite.Run("happy path", func() {
		tx := suite.systemCollection.Transactions[1]
		txID := tx.ID()

		scheduledTxs := []*flow.TransactionBody{
			suite.g.Transactions().Fixture(),
			tx,
			suite.g.Transactions().Fixture(),
		}

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(scheduledTxs, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, actual)
	})

	// Tests the case where the scheduled transaction exists in the indices, but not in the generated
	// system collection (produced from events). This indicates inconsistent state, however it is handled
	// by returning an internal error instead of an irrecoverable since the events may be queried from
	// an Execution node, which could have returned incorrect data.
	suite.Run("not in generated collection", func() {
		tx := suite.systemCollection.Transactions[1]
		txID := tx.ID()

		scheduledTxs := []*flow.TransactionBody{
			suite.g.Transactions().Fixture(),
			suite.g.Transactions().Fixture(),
		}

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(scheduledTxs, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(actual)
	})

	suite.Run("provider error", func() {
		tx := suite.systemCollection.Transactions[1]
		txID := tx.ID()

		expectedErr := status.Errorf(codes.Internal, "some other error")

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(actual)
	})

	suite.Run("block not found exception", func() {
		tx := suite.systemCollection.Transactions[1]
		txID := tx.ID()

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(),
			fmt.Errorf("failed to get block header: %w", storage.ErrNotFound))
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		actual, err := txBackend.GetTransaction(signalerCtx, txID)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(actual)
	})
}

// TestGetTransaction_HistoricalTx tests getting a historical transaction by ID from the historical
// access nodes.
func (suite *Suite) TestGetTransaction_HistoricalTx() {
	suite.Run("historical tx", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(flow.ZeroID, storage.ErrNotFound).
			Once()

		expectedRequest := &access.GetTransactionRequest{
			Id: txID[:],
		}
		response := &access.TransactionResponse{
			Transaction: convert.TransactionToMessage(*tx),
		}

		suite.historicalAccessAPIClient.
			On("GetTransaction", mock.Anything, expectedRequest).
			Return(response, nil)

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, actual)
	})

	suite.Run("historical tx returns unexpected error", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(flow.ZeroID, storage.ErrNotFound).
			Once()

		expectedRequest := &access.GetTransactionRequest{
			Id: txID[:],
		}

		suite.historicalAccessAPIClient.
			On("GetTransaction", mock.Anything, expectedRequest).
			Return(nil, status.Errorf(codes.Internal, "some other error"))

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetTransaction(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(actual)
	})
}

func (suite *Suite) TestGetTransactionResult_SystemTx() {
	tx := suite.systemCollection.Transactions[0]
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	expectedResult := &accessmodel.TransactionResult{
		BlockID:       blockID,
		TransactionID: txID,
		CollectionID:  flow.ZeroID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusExecuted,
	}

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	suite.Run("happy path", func() {
		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(expectedResult, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(res)
		suite.Require().Equal(expectedErr, err)
	})

	suite.Run("block not found exception", func() {
		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block not provided", func() {
		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.InvalidArgument, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetTransactionResult_ScheduledTx() {
	tx := suite.systemCollection.Transactions[1]
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	expectedResult := &accessmodel.TransactionResult{
		BlockID:       blockID,
		TransactionID: txID,
		CollectionID:  flow.ZeroID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusExecuted,
	}

	suite.Run("happy path", func() {
		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(expectedResult, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("scheduled tx lookup failure", func() {
		expectedErr := fmt.Errorf("some other error")

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(flow.ZeroID, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("scheduled tx block mismatch", func() {
		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		incorrectBlockID := suite.g.Identifiers().Fixture()
		res, err := txBackend.GetTransactionResult(context.Background(), txID, incorrectBlockID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("scheduled tx block not found exception", func() {
		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(),
			fmt.Errorf("failed to get scheduled transaction's block from storage: %w", storage.ErrNotFound))
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetTransactionResult(signalerCtx, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(res)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetTransactionResult_SubmittedTx() {
	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	collection := suite.g.Collections().Fixture()
	collectionID := collection.ID()
	lightCollection := collection.Light()

	tx := collection.Transactions[0]
	txID := tx.ID()

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	expectedResult := &accessmodel.TransactionResult{
		BlockID:       blockID,
		BlockHeight:   block.Height,
		TransactionID: txID,
		CollectionID:  collectionID,
		Status:        flow.TransactionStatusExecuted,
	}

	ctx := irrecoverable.NewMockSignalerContext(suite.T(), context.Background())
	ctxNoErr := irrecoverable.WithSignalerContext(context.Background(), ctx)

	suite.scheduledTransactions.
		On("BlockIDByTransactionID", txID).
		Return(flow.ZeroID, storage.ErrNotFound)

	suite.Run("happy path - only txID provided", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, collectionID, encodingVersion).
			Return(expectedResult, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("happy path - all args provided", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, collectionID, encodingVersion).
			Return(expectedResult, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, collectionID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("happy path - not executed", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, collectionID, encodingVersion).
			Return(nil, storage.ErrNotFound)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		expected := *expectedResult
		expected.Status = flow.TransactionStatusFinalized

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, collectionID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expected, *res)
	})

	suite.Run("collection ID mismatch", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		incorrectCollectionID := suite.g.Identifiers().Fixture()
		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, incorrectCollectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block ID mismatch", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		incorrectBlockID := suite.g.Identifiers().Fixture()
		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, incorrectBlockID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("collection lookup error", func() {
		expectedErr := fmt.Errorf("some other error")
		suite.collections.
			On("LightByTransactionID", txID).
			Return(nil, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block lookup notfound returns not found error", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(nil, storage.ErrNotFound).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContext(suite.T(), context.Background())
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetTransactionResult(signalerCtx, txID, blockID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block lookup failure throws exception", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		blockLookupException := fmt.Errorf("collection lookup exception")
		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(nil, blockLookupException).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		expectedErr := fmt.Errorf("failed to find block for collection %v: %w", collectionID, blockLookupException)
		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), expectedErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetTransactionResult(signalerCtx, txID, blockID, collectionID, encodingVersion)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(res)
	})

	suite.Run("tx body lookup failure throws exception", func() {
		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		params := suite.defaultTransactionsParams()
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		expectedErr := fmt.Errorf("failed to get transaction from storage: %w", storage.ErrNotFound)
		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(), expectedErr)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetTransactionResult(signalerCtx, txID, blockID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Nil(res)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		suite.collections.
			On("LightByTransactionID", txID).
			Return(lightCollection, nil).
			Once()

		suite.blocks.
			On("ByCollectionID", collectionID).
			Return(block, nil).
			Once()

		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, collectionID, encodingVersion).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, collectionID, encodingVersion)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetTransactionResult_SubmittedTx_Unknown() {
	blocks := suite.g.Blocks().List(3)

	finalizedBlock := blocks[0]

	refBlock := blocks[1]
	refBlockID := refBlock.ID()

	block := blocks[2]
	blockID := block.ID()

	tx := suite.g.Transactions().Fixture()
	tx.ReferenceBlockID = refBlockID
	txID := tx.ID()

	collection := suite.g.Collections().Fixture()
	lightCollection := collection.Light()
	collectionID := collection.ID()

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	ctx := irrecoverable.NewMockSignalerContext(suite.T(), context.Background())
	ctxNoErr := irrecoverable.WithSignalerContext(context.Background(), ctx)

	suite.scheduledTransactions.
		On("BlockIDByTransactionID", txID).
		Return(flow.ZeroID, storage.ErrNotFound)

	suite.collections.
		On("LightByTransactionID", txID).
		Return(nil, storage.ErrNotFound)

	suite.Run("pending tx", func() {
		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(refBlock.ToHeader(), nil)
		suite.state.On("AtBlockID", refBlockID).Return(snapshot, nil).Once()

		finalSnapshot := protocolmock.NewSnapshot(suite.T())
		finalSnapshot.On("Head").Return(finalizedBlock.ToHeader(), nil)
		suite.state.On("Final").Return(finalSnapshot, nil).Once()

		expectedResult := &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        flow.TransactionStatusPending,
		}

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("transaction lookup error", func() {
		expectedErr := fmt.Errorf("some other error")
		suite.transactions.
			On("ByID", txID).
			Return(nil, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("pending block with derive status exception", func() {
		suite.transactions.
			On("ByID", txID).
			Return(tx, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(refBlock.ToHeader(), nil)
		suite.state.On("AtBlockID", refBlockID).Return(snapshot, nil).Once()

		finalSnapshot := protocolmock.NewSnapshot(suite.T())
		finalSnapshot.On("Head").Return(nil, storage.ErrNotFound)
		suite.state.On("Final").Return(finalSnapshot, nil).Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(),
			fmt.Errorf("failed to derive transaction status: %w", irrecoverable.NewExceptionf("failed to lookup final header: %w", storage.ErrNotFound)))
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetTransactionResult(signalerCtx, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(res)
	})

	suite.Run("unknown tx in known block", func() {
		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		expectedResult := &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        flow.TransactionStatusUnknown,
		}

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("block lookup error", func() {
		expectedErr := fmt.Errorf("some other error")

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.blocks.
			On("ByID", blockID).
			Return(nil, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, blockID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("unknown tx in known collection", func() {
		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.collections.
			On("LightByID", collectionID).
			Return(lightCollection, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("collection lookup error", func() {
		expectedErr := fmt.Errorf("some other error")

		suite.transactions.
			On("ByID", txID).
			Return(nil, storage.ErrNotFound).
			Once()

		suite.collections.
			On("LightByID", collectionID).
			Return(nil, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, collectionID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetTransactionResult_SubmittedTx_HistoricalNodes() {
	tx := suite.g.Transactions().Fixture()
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	expectedFoundResponse := &accessmodel.TransactionResult{
		TransactionID: txID,
		BlockID:       blockID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusSealed,
		Events:        []flow.Event{}, // needed because converter uses an empty slice for nil events
	}

	expectedNotFoundResponse := &accessmodel.TransactionResult{
		TransactionID: txID,
		Status:        flow.TransactionStatusUnknown,
	}

	expectedRequest := &access.GetTransactionRequest{
		Id: txID[:],
	}

	foundResponse := convert.TransactionResultToMessage(expectedFoundResponse)

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	ctx := irrecoverable.NewMockSignalerContext(suite.T(), context.Background())
	ctxNoErr := irrecoverable.WithSignalerContext(context.Background(), ctx)

	suite.scheduledTransactions.
		On("BlockIDByTransactionID", txID).
		Return(flow.ZeroID, storage.ErrNotFound)

	suite.collections.
		On("LightByTransactionID", txID).
		Return(nil, storage.ErrNotFound)

	suite.transactions.
		On("ByID", txID).
		Return(nil, storage.ErrNotFound)

	suite.Run("result found", func() {
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(foundResponse, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxResultCache = new(NoopTxResultCache)
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedFoundResponse, resp)
	})

	suite.Run("result not found", func() {
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.Internal, "some other error")).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxResultCache = new(NoopTxResultCache)
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedNotFoundResponse, resp)
	})

	suite.Run("success result is cached", func() {
		// make sure the cache is starting empty
		suite.txResultCache.Purge()

		// this should only be called once
		// unset to make sure we're definitely testing what we expect
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Unset()
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(foundResponse, nil).
			Once()

		params := suite.defaultTransactionsParams()
		// use real cache (used by default)
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		// first call should populate the cache
		resp, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedFoundResponse, resp)

		// second call should return the cached result
		resp, err = txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedFoundResponse, resp)
	})

	suite.Run("not found result is cached", func() {
		// make sure the cache is starting empty
		suite.txResultCache.Purge()

		// this should only be called once
		// unset to make sure we're definitely testing what we expect
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Unset()
		suite.historicalAccessAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.Internal, "some other error")).
			Once()

		params := suite.defaultTransactionsParams()
		// use real cache (used by default)
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		// first call should populate the cache
		resp, err := txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedNotFoundResponse, resp)

		// second call should return the cached result
		resp, err = txBackend.GetTransactionResult(ctxNoErr, txID, flow.ZeroID, flow.ZeroID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedNotFoundResponse, resp)
	})
}

func (suite *Suite) TestGetHistoricalTransactionResult() {
	tx := suite.g.Transactions().Fixture()
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	expectedFoundResponse := &accessmodel.TransactionResult{
		TransactionID: txID,
		BlockID:       blockID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusSealed,
		Events:        []flow.Event{}, // needed because converter uses an empty slice for nil events
	}
	response := convert.TransactionResultToMessage(expectedFoundResponse)

	expectedRequest := &access.GetTransactionRequest{
		Id: txID[:],
	}

	an1 := accessmock.NewAccessAPIClient(suite.T())
	an2 := accessmock.NewAccessAPIClient(suite.T())
	an3 := accessmock.NewAccessAPIClient(suite.T())

	suite.Run("iterates through multiple nodes", func() {
		an1.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.NotFound, "not found")).
			Once()

		an2.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.Unavailable, "unavailable")).
			Once()

		an3.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(response, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{an1, an2, an3}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.getHistoricalTransactionResult(context.Background(), txID)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedFoundResponse, resp)
	})

	suite.Run("returns not found when all nodes return not found", func() {
		an1.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.OutOfRange, "out of range")).
			Once()

		an2.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.Unavailable, "unavailable")).
			Once()

		an3.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(nil, status.Error(codes.Internal, "internal error")).
			Once()

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{an1, an2, an3}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.getHistoricalTransactionResult(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(resp)
	})

	suite.Run("returns unknown when result status is unknown", func() {
		expectedResponse := &accessmodel.TransactionResult{
			TransactionID: txID,
			Status:        flow.TransactionStatusUnknown,
		}
		response := convert.TransactionResultToMessage(expectedResponse)

		an1.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(response, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{an1}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.getHistoricalTransactionResult(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(resp)
	})

	suite.Run("returns expired when result status is pending", func() {
		nodeResponse := &accessmodel.TransactionResult{
			TransactionID: txID,
			BlockID:       blockID,
			BlockHeight:   block.Height,
			Status:        flow.TransactionStatusPending,
		}
		response := convert.TransactionResultToMessage(nodeResponse)

		// result status should be updated to expired
		expectedResponse := &accessmodel.TransactionResult{
			TransactionID: txID,
			BlockID:       blockID,
			BlockHeight:   block.Height,
			Status:        flow.TransactionStatusExpired,
			Events:        []flow.Event{}, // needed because converter uses an empty slice for nil events
		}

		an1.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(response, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{an1}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.getHistoricalTransactionResult(context.Background(), txID)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResponse, resp)
	})

	suite.Run("returns error when convert fails", func() {
		nodeResponse := &accessmodel.TransactionResult{
			TransactionID: txID,
			BlockID:       blockID,
			BlockHeight:   block.Height,
			Status:        flow.TransactionStatusSealed,
			Events: []flow.Event{
				suite.g.Events().Fixture(
					// this is invalid and will cause the converter to fail
					suite.g.Events().WithTransactionID(flow.ZeroID),
				),
			},
		}
		response := convert.TransactionResultToMessage(nodeResponse)

		an1.On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(response, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.HistoricalAccessNodeClients = []access.AccessAPIClient{an1}
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		resp, err := txBackend.getHistoricalTransactionResult(context.Background(), txID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(resp)
	})
}

func (suite *Suite) TestGetSystemTransaction() {
	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	suite.Run("returns process transactions transaction", func() {
		tx := suite.systemCollection.Transactions[0]
		txID := tx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Once()
		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		res, err := txBackend.GetSystemTransaction(context.Background(), txID, blockID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, res)
	})

	suite.Run("returns system chunk transaction", func() {
		tx := suite.systemCollection.Transactions[len(suite.systemCollection.Transactions)-1]
		txID := tx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)
		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		res, err := txBackend.GetSystemTransaction(context.Background(), txID, blockID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, res)
	})

	suite.Run("returns system chunk transaction when tx is not provided", func() {
		tx := suite.systemCollection.Transactions[len(suite.systemCollection.Transactions)-1]

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)
		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		res, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, res)
	})

	suite.Run("returns error when block not found", func() {
		tx := suite.systemCollection.Transactions[0]
		txID := tx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)
		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		res, err := txBackend.GetSystemTransaction(context.Background(), txID, blockID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("returns not found for non-system transaction", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Once()
		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		res, err := txBackend.GetSystemTransaction(context.Background(), txID, blockID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetSystemTransactionResult() {
	tx := suite.systemCollection.Transactions[0]
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	expectedResult := &accessmodel.TransactionResult{
		BlockID:       blockID,
		TransactionID: txID,
		CollectionID:  flow.ZeroID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusExecuted,
	}

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	suite.Run("happy path", func() {
		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Times(2)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Times(2)

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(expectedResult, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), txID, blockID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("uses system chunk tx when txID is not provided", func() {
		systemTx := suite.systemCollection.Transactions[len(suite.systemCollection.Transactions)-1]
		systemTxID := systemTx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Times(2)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Times(2)

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), systemTxID, flow.ZeroID, encodingVersion).
			Return(expectedResult, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), flow.ZeroID, blockID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("returns not found when tx is not a system tx", func() {
		tx := suite.g.Transactions().Fixture()
		txID := tx.ID()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Once()

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), txID, blockID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Times(2)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Times(2)

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(nil, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), txID, blockID, encodingVersion)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(res)
		suite.Require().Equal(expectedErr, err)
	})

	suite.Run("block not found exception", func() {
		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), txID, blockID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block not provided", func() {
		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil).Once()

		suite.state.
			On("AtBlockID", flow.ZeroID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransactionResult(context.Background(), txID, flow.ZeroID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.InvalidArgument, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetScheduledTransaction() {
	scheduledTxID := suite.g.Random().Uint64()
	tx := suite.systemCollection.Transactions[1]
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	suite.Run("happy path", func() {
		scheduledTxs := []*flow.TransactionBody{
			suite.g.Transactions().Fixture(),
			tx,
			suite.g.Transactions().Fixture(),
		}

		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(scheduledTxs, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		suite.Require().NoError(err)
		suite.Require().Equal(tx, actual)
	})

	suite.Run("not a scheduled tx", func() {
		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(nil, storage.ErrNotFound).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(actual)
	})

	// Tests the case where the scheduled transaction exists in the indices, but not in the generated
	// system collection (produced from events). This indicates inconsistent state, however it is handled
	// by returning an internal error instead of an irrecoverable since the events may be queried from
	// an Execution node, which could have returned incorrect data.
	suite.Run("not in generated collection", func() {
		scheduledTxs := []*flow.TransactionBody{
			suite.g.Transactions().Fixture(),
			suite.g.Transactions().Fixture(),
		}

		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(scheduledTxs, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(actual)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("ScheduledTransactionsByBlockID", mock.Anything, block.ToHeader()).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		actual, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(actual)
	})

	suite.Run("block not found exception", func() {
		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(),
			fmt.Errorf("failed to get block header: %w", storage.ErrNotFound))
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		actual, err := txBackend.GetScheduledTransaction(signalerCtx, scheduledTxID)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(actual)
	})

	suite.Run("returns unimplemented error when scheduled transactions are not enabled", func() {
		params := suite.defaultTransactionsParams()
		params.ScheduledTransactions = nil

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransaction(context.Background(), 0)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Unimplemented, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetScheduledTransactionResult() {
	scheduledTxID := suite.g.Random().Uint64()
	tx := suite.systemCollection.Transactions[1]
	txID := tx.ID()

	block := suite.g.Blocks().Fixture()
	blockID := block.ID()

	encodingVersion := entities.EventEncodingVersion_JSON_CDC_V0

	expectedResult := &accessmodel.TransactionResult{
		BlockID:       blockID,
		TransactionID: txID,
		CollectionID:  flow.ZeroID,
		BlockHeight:   block.Height,
		Status:        flow.TransactionStatusExecuted,
	}

	suite.Run("happy path", func() {
		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(expectedResult, nil)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransactionResult(context.Background(), scheduledTxID, encodingVersion)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedResult, res)
	})

	suite.Run("not a scheduled tx", func() {
		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(nil, storage.ErrNotFound).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransactionResult(context.Background(), scheduledTxID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("scheduled tx by block ID lookup failure", func() {
		expectedErr := fmt.Errorf("some other error")

		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(flow.ZeroID, expectedErr).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransactionResult(context.Background(), scheduledTxID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Nil(res)
	})

	suite.Run("block not found exception", func() {
		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(nil, storage.ErrNotFound)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		params := suite.defaultTransactionsParams()
		params.TxProvider = providermock.NewTransactionProvider(suite.T())

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		ctx := irrecoverable.NewMockSignalerContextExpectError(suite.T(), context.Background(),
			fmt.Errorf("failed to get scheduled transaction's block from storage: %w", storage.ErrNotFound))
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), ctx)

		res, err := txBackend.GetScheduledTransactionResult(signalerCtx, scheduledTxID, encodingVersion)
		suite.Require().Error(err) // specific error doen't matter since it's thrown
		suite.Require().Nil(res)
	})

	suite.Run("provider error", func() {
		expectedErr := status.Errorf(codes.Internal, "some other error")

		suite.scheduledTransactions.
			On("TransactionIDByID", scheduledTxID).
			Return(txID, nil).
			Once()

		suite.scheduledTransactions.
			On("BlockIDByTransactionID", txID).
			Return(blockID, nil).
			Once()

		snapshot := protocolmock.NewSnapshot(suite.T())
		snapshot.On("Head").Return(block.ToHeader(), nil)

		suite.state.
			On("AtBlockID", blockID).
			Return(snapshot, nil).
			Once()

		provider := providermock.NewTransactionProvider(suite.T())
		provider.
			On("TransactionResult", mock.Anything, block.ToHeader(), txID, flow.ZeroID, encodingVersion).
			Return(nil, expectedErr)

		params := suite.defaultTransactionsParams()
		params.TxProvider = provider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransactionResult(context.Background(), scheduledTxID, encodingVersion)
		suite.Require().ErrorIs(err, expectedErr)
		suite.Require().Nil(res)
	})

	suite.Run("returns unimplemented error when scheduled transactions are not enabled", func() {
		params := suite.defaultTransactionsParams()
		params.ScheduledTransactions = nil

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetScheduledTransactionResult(context.Background(), scheduledTxID, encodingVersion)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Unimplemented, status.Code(err))
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestSendTransaction() {
	refBlock := suite.g.Headers().Fixture()
	refBlockID := refBlock.ID()
	finalBlock := suite.g.Headers().Fixture(fixtures.Header.WithParentHeader(refBlock))
	tx := suite.g.Transactions().Fixture(fixtures.Transaction.WithReferenceBlockID(refBlockID))

	expectedRequest := &access.SendTransactionRequest{
		Transaction: convert.TransactionToMessage(*tx),
	}

	response := &access.SendTransactionResponse{
		Id: convert.IdentifierToMessage(tx.ID()),
	}

	nodes := suite.g.Identities().List(2, fixtures.Identity.WithRole(flow.RoleCollection))

	suite.Run("happy path", func() {
		client := suite.addCollectionClient(nodes[0].Address)
		client.On("SendTransaction", mock.Anything, expectedRequest).Return(response, nil)

		suite.transactions.On("Store", tx).Return(nil).Once()

		finalSnapshot := protocolmock.NewSnapshot(suite.T())
		finalSnapshot.On("Head").Return(finalBlock, nil).Once()
		suite.state.On("Final").Return(finalSnapshot, nil).Twice() // once to get clustering, once to validate the tx

		refBlockSnapshot := protocolmock.NewSnapshot(suite.T())
		refBlockSnapshot.On("Head").Return(refBlock, nil).Once()
		suite.state.On("AtBlockID", refBlockID).Return(refBlockSnapshot, nil).Once()

		configureClustering(suite.T(), flow.IdentityList{nodes[0]}, finalSnapshot)

		params := suite.defaultTransactionsParams()
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		err = txBackend.SendTransaction(context.Background(), tx)
		suite.Require().NoError(err)
	})

	// test that the correct node is used when a static collection node is provided and the
	// clustering lookup is never performed
	suite.Run("static collection node", func() {
		client := accessmock.NewAccessAPIClient(suite.T())
		client.On("SendTransaction", mock.Anything, expectedRequest).Return(response, nil)

		finalSnapshot := protocolmock.NewSnapshot(suite.T())
		finalSnapshot.On("Head").Return(finalBlock, nil).Once()
		suite.state.On("Final").Return(finalSnapshot, nil).Once()

		refBlockSnapshot := protocolmock.NewSnapshot(suite.T())
		refBlockSnapshot.On("Head").Return(refBlock, nil).Once()
		suite.state.On("AtBlockID", refBlockID).Return(refBlockSnapshot, nil).Once()

		suite.transactions.On("Store", tx).Return(nil).Once()

		params := suite.defaultTransactionsParams()
		params.StaticCollectionRPCClient = client

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		err = txBackend.SendTransaction(context.Background(), tx)
		suite.Require().NoError(err)
	})

	// test that each collection node is tried when requests fail
	suite.Run("multiple collection nodes tried when first fails", func() {
		expectedErr := status.Errorf(codes.Internal, "test failed to send transaction")

		for _, node := range nodes {
			client := suite.addCollectionClient(node.Address)
			client.On("SendTransaction", mock.Anything, expectedRequest).Return(nil, expectedErr)
		}

		finalSnapshot := protocolmock.NewSnapshot(suite.T())
		finalSnapshot.On("Head").Return(finalBlock, nil).Once()
		suite.state.On("Final").Return(finalSnapshot, nil).Twice() // once to get clustering, once to validate the tx

		refBlockSnapshot := protocolmock.NewSnapshot(suite.T())
		refBlockSnapshot.On("Head").Return(refBlock, nil).Once()
		suite.state.On("AtBlockID", refBlockID).Return(refBlockSnapshot, nil).Once()

		configureClustering(suite.T(), nodes, finalSnapshot)

		params := suite.defaultTransactionsParams()
		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		err = txBackend.SendTransaction(context.Background(), tx)
		suite.Require().Error(err)
		suite.Require().Equal(codes.Internal, status.Code(err))
		suite.Require().Contains(err.Error(), expectedErr.Error())
	})
}

func configureClustering(t *testing.T, identities flow.IdentityList, finalSnapshot *protocolmock.Snapshot) {
	epoch := protocolmock.NewCommittedEpoch(t)
	epoch.On("Clustering").Return(flow.ClusterList{identities.ToSkeleton()}, nil).Once()

	epochQuery := protocolmock.NewEpochQuery(t)
	epochQuery.On("Current").Return(epoch, nil).Once()

	finalSnapshot.On("Epochs").Return(epochQuery, nil).Once()
}

func (suite *Suite) addCollectionClient(address string) *accessmock.AccessAPIClient {
	client := accessmock.NewAccessAPIClient(suite.T())
	suite.connectionFactory.
		On("GetCollectionAPIClient", address, mock.Anything).
		Return(client, &mocks.MockCloser{}, nil).
		Once()

	return client
}
