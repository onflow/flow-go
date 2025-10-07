package transactions

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/cadence"
	cadenceCommon "github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/validator"
	validatormock "github.com/onflow/flow-go/access/validator/mock"
	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/retrier"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/system"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/metrics"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

const expectedErrorMsg = "expected test error"

type Suite struct {
	suite.Suite

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
	txResultCache         *lru.Cache[flow.Identifier, *accessmodel.TransactionResult]

	db                  *pebble.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter

	executionAPIClient        *accessmock.ExecutionAPIClient
	historicalAccessAPIClient *accessmock.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory

	reporter       *syncmock.IndexReporter
	indexReporter  *index.Reporter
	eventsIndex    *index.EventsIndex
	txResultsIndex *index.TransactionResultsIndex

	errorMessageProvider error_messages.Provider

	chainID                              flow.ChainID
	defaultSystemCollection              *system.SystemCollection
	systemCollection                     *flow.Collection
	pendingExecutionEvents               []flow.Event
	processScheduledTransactionEventType flow.EventType
	scheduledTransactionsEnabled         bool

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestTransactionsBackend(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = unittest.Logger()
	suite.snapshot = protocolmock.NewSnapshot(suite.T())

	header := unittest.BlockHeaderFixture()
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
	suite.chainID = flow.Testnet
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

	// this is the system collection with no scheduled transactions used within the backend
	suite.defaultSystemCollection, err = system.DefaultSystemCollection(suite.chainID)
	suite.Require().NoError(err)
	suite.scheduledTransactionsEnabled = true

	// this is the system collection with scheduled transactions used as block data
	suite.pendingExecutionEvents = suite.createPendingExecutionEvents(2) // 2 callbacks
	suite.systemCollection, err = blueprints.SystemCollection(suite.chainID.Chain(), suite.pendingExecutionEvents)
	suite.Require().NoError(err)
	suite.processScheduledTransactionEventType = suite.pendingExecutionEvents[0].Type

	suite.db, suite.dbDir = unittest.TempPebbleDB(suite.T())
	progress, err := store.NewConsumerProgress(pebbleimpl.ToDB(suite.db), module.ConsumeProgressLastFullBlockHeight).Initialize(0)
	require.NoError(suite.T(), err)
	suite.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	suite.Require().NoError(err)

	suite.fixedExecutionNodeIDs = nil
	suite.preferredExecutionNodeIDs = nil
	suite.errorMessageProvider = nil
}

func (suite *Suite) TearDownTest() {
	err := os.RemoveAll(suite.dbDir)
	suite.Require().NoError(err)
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

	txValidator, err := validator.NewTransactionValidator(
		validatormock.NewBlocks(suite.T()),
		suite.chainID.Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{},
		execmock.NewScriptExecutor(suite.T()),
	)
	suite.Require().NoError(err)

	nodeCommunicator := node_communicator.NewNodeCommunicator(false)

	txProvider := provider.NewENTransactionProvider(
		suite.log,
		suite.state,
		suite.collections,
		suite.connectionFactory,
		nodeCommunicator,
		nodeProvider,
		txStatusDeriver,
		suite.defaultSystemCollection,
		suite.chainID,
		suite.scheduledTransactionsEnabled,
	)

	return Params{
		Log:                          suite.log,
		Metrics:                      metrics.NewNoopCollector(),
		State:                        suite.state,
		ChainID:                      flow.Testnet,
		SystemCollection:             suite.defaultSystemCollection,
		StaticCollectionRPCClient:    suite.historicalAccessAPIClient,
		HistoricalAccessNodeClients:  nil,
		NodeCommunicator:             nodeCommunicator,
		ConnFactory:                  suite.connectionFactory,
		EnableRetries:                true,
		NodeProvider:                 nodeProvider,
		Blocks:                       suite.blocks,
		Collections:                  suite.collections,
		Transactions:                 suite.transactions,
		Events:                       suite.events,
		TxErrorMessageProvider:       suite.errorMessageProvider,
		TxResultCache:                suite.txResultCache,
		TxProvider:                   txProvider,
		TxValidator:                  txValidator,
		TxStatusDeriver:              txStatusDeriver,
		EventsIndex:                  suite.eventsIndex,
		TxResultsIndex:               suite.txResultsIndex,
		ScheduledTransactionsEnabled: suite.scheduledTransactionsEnabled,
	}
}

// TestGetTransactionResult_UnknownTx returns unknown result when tx not found
func (suite *Suite) TestGetTransactionResult_UnknownTx() {
	block := unittest.BlockFixture()
	tx := unittest.TransactionBodyFixture()
	coll := unittest.CollectionFromTransactions(&tx)

	suite.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)
	res, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(res.Status, flow.TransactionStatusUnknown)
	suite.Require().Empty(res.BlockID)
	suite.Require().Empty(res.BlockHeight)
	suite.Require().Empty(res.TransactionID)
	suite.Require().Empty(res.CollectionID)
	suite.Require().Empty(res.ErrorMessage)
}

// TestGetTransactionResult_TxLookupFailure returns error from transaction storage
func (suite *Suite) TestGetTransactionResult_TxLookupFailure() {
	block := unittest.BlockFixture()
	tx := unittest.TransactionBodyFixture()
	coll := unittest.CollectionFromTransactions(&tx)

	expectedErr := fmt.Errorf("some other error")
	suite.transactions.
		On("ByID", tx.ID()).
		Return(nil, expectedErr)

	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)

	_, err = txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().Equal(status.Errorf(codes.Internal, "failed to lookup transaction: %v", expectedErr), err)
}

// TestGetTransactionResult_HistoricNodes_Success tests lookup in historic nodes
func (suite *Suite) TestGetTransactionResult_HistoricNodes_Success() {
	block := unittest.BlockFixture()
	tx := unittest.TransactionBodyFixture()
	coll := unittest.CollectionFromTransactions(&tx)

	suite.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	transactionResultResponse := access.TransactionResultResponse{
		Status:     entities.TransactionStatus_EXECUTED,
		StatusCode: uint32(entities.TransactionStatus_EXECUTED),
	}

	suite.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(&transactionResultResponse, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)

	resp, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
	suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)
}

// TestGetTransactionResult_HistoricNodes_FromCache get historic transaction result from cache
func (suite *Suite) TestGetTransactionResult_HistoricNodes_FromCache() {
	block := unittest.BlockFixture()
	tx := unittest.TransactionBodyFixture()

	suite.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	transactionResultResponse := access.TransactionResultResponse{
		Status:     entities.TransactionStatus_EXECUTED,
		StatusCode: uint32(entities.TransactionStatus_EXECUTED),
	}

	suite.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(&transactionResultResponse, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)

	coll := unittest.CollectionFromTransactions(&tx)
	resp, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
	suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)

	resp2, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(flow.TransactionStatusExecuted, resp2.Status)
	suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp2.StatusCode)
}

// TestGetTransactionResultUnknownFromCache retrieve unknown result from cache.
func (suite *Suite) TestGetTransactionResultUnknownFromCache() {
	block := unittest.BlockFixture()
	tx := unittest.TransactionBodyFixture()

	suite.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	suite.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).
		Once()

	params := suite.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{suite.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)

	coll := unittest.CollectionFromTransactions(&tx)
	resp, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(flow.TransactionStatusUnknown, resp.Status)
	suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp.StatusCode)

	// ensure the unknown transaction is cached when not found anywhere
	txStatus := flow.TransactionStatusUnknown
	res, ok := txBackend.txResultCache.Get(tx.ID())
	suite.Require().True(ok)
	suite.Require().Equal(res, &accessmodel.TransactionResult{
		Status:     txStatus,
		StatusCode: uint(txStatus),
	})

	// ensure underlying GetTransactionResult() won't be called the second time
	resp2, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(flow.TransactionStatusUnknown, resp2.Status)
	suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp2.StatusCode)
}

// TestGetSystemTransaction_HappyPath tests that GetSystemTransaction call returns system chunk transaction.
func (suite *Suite) TestGetSystemTransaction_ExecutionNode_HappyPath() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	params := suite.defaultTransactionsParams()
	enabledProvider := provider.NewENTransactionProvider(
		suite.log,
		suite.state,
		suite.collections,
		suite.connectionFactory,
		params.NodeCommunicator,
		params.NodeProvider,
		params.TxStatusDeriver,
		suite.defaultSystemCollection,
		suite.chainID,
		true,
	)
	disabledProvider := provider.NewENTransactionProvider(
		suite.log,
		suite.state,
		suite.collections,
		suite.connectionFactory,
		params.NodeCommunicator,
		params.NodeProvider,
		params.TxStatusDeriver,
		suite.defaultSystemCollection,
		suite.chainID,
		false,
	)

	suite.params.On("FinalizedRoot").Unset()

	suite.Run("scheduled callbacks DISABLED - ZeroID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks DISABLED - system txID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks DISABLED - non-system txID fails", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().Error(err)
		suite.Require().Nil(res)
	})

	suite.Run("scheduled callbacks ENABLED - ZeroID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks ENABLED - system txID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks ENABLED - system collection TX", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		// get execution node identities
		suite.params.On("FinalizedRoot").Return(block.ToHeader(), nil)
		suite.state.On("Final").Return(suite.snapshot, nil).Twice()
		suite.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil).Twice()

		suite.setupExecutionGetEventsRequest(blockID, block.Height, suite.pendingExecutionEvents)

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks ENABLED - non-system txID fails", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		suite.params.On("FinalizedRoot").Return(block.ToHeader(), nil)
		suite.state.On("Final").Return(suite.snapshot, nil).Twice()
		suite.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil).Twice()

		suite.setupExecutionGetEventsRequest(blockID, block.Height, suite.pendingExecutionEvents)

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID)
		suite.Require().Error(err)
		suite.Require().Nil(res)
	})
}

// TestGetSystemTransaction_HappyPath tests that GetSystemTransaction call returns system chunk transaction.
func (suite *Suite) TestGetSystemTransaction_Local_HappyPath() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	params := suite.defaultTransactionsParams()
	enabledProvider := provider.NewLocalTransactionProvider(
		suite.state,
		suite.collections,
		suite.blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		suite.defaultSystemCollection,
		params.TxStatusDeriver,
		suite.chainID,
		true,
	)
	disabledProvider := provider.NewLocalTransactionProvider(
		suite.state,
		suite.collections,
		suite.blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		suite.defaultSystemCollection,
		params.TxStatusDeriver,
		suite.chainID,
		false,
	)

	suite.params.On("FinalizedRoot").Unset()

	suite.Run("scheduled callbacks DISABLED - ZeroID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks DISABLED - system txID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks DISABLED - non-system txID fails", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID)
		suite.Require().Error(err)
		suite.Require().Nil(res)
	})

	suite.Run("scheduled callbacks ENABLED - ZeroID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks ENABLED - system txID", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), suite.defaultSystemCollection.SystemTxID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(suite.defaultSystemCollection.SystemTx(), res)
	})

	suite.Run("scheduled callbacks ENABLED - system collection TX", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil).Once()
		suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil).Once()
		suite.events.On("ByBlockID", blockID).Return(suite.pendingExecutionEvents, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		systemTx := suite.systemCollection.Transactions[2]
		res, err := txBackend.GetSystemTransaction(context.Background(), systemTx.ID(), blockID)
		suite.Require().NoError(err)

		suite.Require().Equal(systemTx, res)
	})

	suite.Run("scheduled callbacks ENABLED - non-system txID fails", func() {
		suite.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil).Once()
		suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil).Once()
		suite.events.On("ByBlockID", blockID).Return(suite.pendingExecutionEvents, nil).Once()

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID)
		suite.Require().Error(err)
		suite.Require().Nil(res)
	})
}

func (suite *Suite) TestGetSystemTransactionResult_ExecutionNode_HappyPath() {
	test := func(snapshot protocol.Snapshot) {
		suite.state.
			On("Sealed").
			Return(snapshot, nil).
			Once()

		lastBlock, err := snapshot.Head()
		suite.Require().NoError(err)

		identities, err := snapshot.Identities(filter.Any)
		suite.Require().NoError(err)

		block := unittest.BlockWithParentFixture(lastBlock)
		blockID := block.ID()
		suite.state.
			On("AtBlockID", blockID).
			Return(unittest.StateSnapshotForKnownBlock(block.ToHeader(), identities.Lookup()), nil).
			Once()

		// block storage returns the corresponding block
		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		receipt1 := unittest.ReceiptForBlockFixture(block)
		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(flow.ExecutionReceiptList{receipt1}, nil)

		// Generating events with event generator
		exeNodeEventEncodingVersion := entities.EventEncodingVersion_CCF_V0
		events := unittest.EventGenerator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
		eventMessages := convert.EventsToMessages(events)

		systemTxID := suite.defaultSystemCollection.SystemTxID()
		expectedRequest := &execproto.GetTransactionResultRequest{
			BlockId:       blockID[:],
			TransactionId: systemTxID[:],
		}
		exeEventResp := &execproto.GetTransactionResultResponse{
			Events:               eventMessages,
			EventEncodingVersion: exeNodeEventEncodingVersion,
		}

		suite.executionAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(exeEventResp, nil).
			Once()

		suite.connectionFactory.
			On("GetExecutionAPIClient", mock.Anything).
			Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
			Once()

		// the connection factory should be used to get the execution node client
		params := suite.defaultTransactionsParams()
		backend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		res, err := backend.GetSystemTransactionResult(
			context.Background(),
			flow.ZeroID,
			block.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)

		// Expected system chunk transaction
		suite.Require().Equal(flow.TransactionStatusExecuted, res.Status)
		suite.Require().Equal(systemTxID, res.TransactionID)

		// Check for successful decoding of event
		_, err = jsoncdc.Decode(nil, res.Events[0].Payload)
		suite.Require().NoError(err)

		events, err = convert.MessagesToEventsWithEncodingConversion(
			eventMessages,
			exeNodeEventEncodingVersion,
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(events, res.Events)
	}

	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(
		suite.T(),
		rootSnapshot,
		func(db storage.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
			epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)

			epochBuilder.
				BuildEpoch().
				CompleteEpoch()

			// get heights of each phase in built epochs
			epoch1, ok := epochBuilder.EpochHeights(1)
			require.True(suite.T(), ok)

			snapshot := state.AtHeight(epoch1.FinalHeight())
			suite.state.On("Final").Return(snapshot)
			test(snapshot)
		},
	)
}

func (suite *Suite) TestGetSystemTransactionResult_Local_HappyPath() {
	block := unittest.BlockFixture()
	sysTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
	suite.Require().NoError(err)
	suite.Require().NotNil(sysTx)
	txId := suite.defaultSystemCollection.SystemTxID()
	blockId := block.ID()

	suite.blocks.
		On("ByID", blockId).
		Return(block, nil).
		Once()

	lightTxShouldFail := false
	suite.lightTxResults.
		On("ByBlockIDTransactionID", blockId, txId).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          lightTxShouldFail,
			ComputationUsed: 0,
		}, nil).
		Once()

	// Set up the events storage mock
	var eventsForTx []flow.Event
	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	// create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Height+10, nil)

	indexReporter := index.NewReporter()
	err = indexReporter.Initialize(reporter)
	suite.Require().NoError(err)

	// Set up the backend parameters and the backend instance
	params := suite.defaultTransactionsParams()
	params.EventsIndex = index.NewEventsIndex(indexReporter, suite.events)
	params.TxResultsIndex = index.NewTransactionResultsIndex(indexReporter, suite.lightTxResults)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		params.SystemCollection,
		params.TxStatusDeriver,
		params.ChainID,
		params.ScheduledTransactionsEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)
	response, err := txBackend.GetSystemTransactionResult(context.Background(), flow.ZeroID, blockId, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, *block, txId, lightTxShouldFail, eventsForTx)
}

// TestGetSystemTransactionResult_BlockNotFound tests GetSystemTransactionResult function when block was not found.
func (suite *Suite) TestGetSystemTransactionResult_BlockNotFound() {
	block := unittest.BlockFixture()
	suite.blocks.
		On("ByID", block.ID()).
		Return(nil, storage.ErrNotFound).
		Once()

	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)
	res, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		flow.ZeroID,
		block.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
	)

	suite.Require().Nil(res)
	suite.Require().Error(err)
	suite.Require().Equal(err, status.Errorf(codes.NotFound, "not found: %v", fmt.Errorf("key not found")))
}

// TestGetSystemTransactionResult_FailedEncodingConversion tests the GetSystemTransactionResult function with different
// event encoding versions.
func (suite *Suite) TestGetSystemTransactionResult_FailedEncodingConversion() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	suite.snapshot.On("Head").Return(block.ToHeader(), nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.state.On("Final").Return(suite.snapshot, nil)

	// block storage returns the corresponding block
	suite.blocks.
		On("ByID", blockID).
		Return(block, nil).
		Once()

	// create empty events
	eventsPerBlock := 10
	eventMessages := make([]*entities.Event, eventsPerBlock)

	systemTxID := suite.defaultSystemCollection.SystemTxID()
	expectedRequest := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: systemTxID[:],
	}
	exeEventResp := &execproto.GetTransactionResultResponse{
		Events:               eventMessages,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	}

	suite.executionAPIClient.
		On("GetTransactionResult", mock.Anything, expectedRequest).
		Return(exeEventResp, nil).
		Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	res, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		flow.ZeroID,
		block.ID(),
		entities.EventEncodingVersion_CCF_V0,
	)

	suite.Require().Nil(res)
	suite.Require().Error(err)
	suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to convert events to message: %v",
		fmt.Errorf("conversion from format JSON_CDC_V0 to CCF_V0 is not supported")))
}

// TestGetTransactionResult_FromStorage tests the retrieval of a transaction result (flow.TransactionResult) from storage
// instead of requesting it from the Execution Node.
func (suite *Suite) TestGetTransactionResult_FromStorage() {
	// Create fixtures for block, transaction, and collection
	transaction := unittest.TransactionBodyFixture()
	col := unittest.CollectionFromTransactions(&transaction)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	txId := transaction.ID()
	blockId := block.ID()

	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	suite.lightTxResults.On("ByBlockIDTransactionID", blockId, txId).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil)

	suite.transactions.
		On("ByID", txId).
		Return(&transaction, nil)

	// Set up the light collection and mock the behavior of the collections object
	lightCol := col.Light()
	suite.collections.On("LightByID", col.ID()).Return(lightCol, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}
	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	suite.state.On("Final").Return(suite.snapshot, nil)
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil)
	suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil)

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	// Set up the expected error message for the execution node response
	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       blockId[:],
		TransactionId: txId[:],
	}
	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: txId[:],
		ErrorMessage:  expectedErrorMsg,
	}
	suite.executionAPIClient.
		On("GetTransactionErrorMessage", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.TxErrorMessageProvider = error_messages.NewTxErrorMessageProvider(
		params.Log,
		nil,
		params.TxResultsIndex,
		params.ConnFactory,
		params.NodeCommunicator,
		params.NodeProvider,
	)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		params.SystemCollection,
		params.TxStatusDeriver,
		params.ChainID,
		params.ScheduledTransactionsEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	response, err := txBackend.GetTransactionResult(context.Background(), txId, blockId, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, *block, txId, true, eventsForTx)

	suite.reporter.AssertExpectations(suite.T())
	suite.connectionFactory.AssertExpectations(suite.T())
	suite.executionAPIClient.AssertExpectations(suite.T())
	suite.blocks.AssertExpectations(suite.T())
	suite.events.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
}

// TestTransactionByIndexFromStorage tests the retrieval of a transaction result (flow.TransactionResult) by index
// and returns it from storage instead of requesting from the Execution Node.
func (suite *Suite) TestTransactionByIndexFromStorage() {
	// Create fixtures for block, transaction, and collection
	transaction := unittest.TransactionBodyFixture()
	col := unittest.CollectionFromTransactions(&transaction)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()
	txId := transaction.ID()
	txIndex := rand.Uint32()

	// Set up the light collection and mock the behavior of the collections object
	lightCol := col.Light()
	suite.collections.On("LightByID", col.ID()).Return(lightCol, nil)

	// Mock the behavior of the blocks and lightTxResults objects
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)

	suite.lightTxResults.On("ByBlockIDTransactionIndex", blockId, txIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionIndex", blockId, txIndex).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()
	suite.state.On("Final").Return(suite.snapshot, nil)
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil)
	suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil)

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.TxErrorMessageProvider = error_messages.NewTxErrorMessageProvider(
		params.Log,
		nil,
		params.TxResultsIndex,
		params.ConnFactory,
		params.NodeCommunicator,
		params.NodeProvider,
	)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		params.SystemCollection,
		params.TxStatusDeriver,
		params.ChainID,
		params.ScheduledTransactionsEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response
	exeEventReq := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: blockId[:],
		Index:   txIndex,
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: txId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.executionAPIClient.
		On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	response, err := txBackend.GetTransactionResultByIndex(context.Background(), blockId, txIndex, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, *block, txId, true, eventsForTx)
}

// TestTransactionResultsByBlockIDFromStorage tests the retrieval of transaction results ([]flow.TransactionResult)
// by block ID from storage instead of requesting from the Execution Node.
func (suite *Suite) TestTransactionResultsByBlockIDFromStorage() {
	// Create fixtures for the block and collection
	col := unittest.CollectionFixture(2)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()

	// Mock the behavior of the blocks, collections and light transaction results objects
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)
	lightCol := col.Light()
	suite.collections.
		On("LightByID", mock.Anything).
		Return(lightCol, nil).
		Once()

	lightTxResults := make([]flow.LightTransactionResult, len(lightCol.Transactions))
	for i, txID := range lightCol.Transactions {
		lightTxResults[i] = flow.LightTransactionResult{
			TransactionID:   txID,
			Failed:          false,
			ComputationUsed: 0,
		}
	}
	// simulate the system tx
	lightTxResults = append(lightTxResults, flow.LightTransactionResult{
		TransactionID:   suite.defaultSystemCollection.SystemTxID(),
		Failed:          false,
		ComputationUsed: 10,
	})

	// Mark the first transaction as failed
	lightTxResults[0].Failed = true
	suite.lightTxResults.
		On("ByBlockID", blockId).
		Return(lightTxResults, nil).
		Once()

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.
		On("ByBlockIDTransactionID", blockId, mock.Anything).
		Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()
	suite.state.On("Final").Return(suite.snapshot, nil)
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil)
	suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil)

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.TxErrorMessageProvider = error_messages.NewTxErrorMessageProvider(
		params.Log,
		nil,
		params.TxResultsIndex,
		params.ConnFactory,
		params.NodeCommunicator,
		params.NodeProvider,
	)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.EventsIndex,
		params.TxResultsIndex,
		params.TxErrorMessageProvider,
		params.SystemCollection,
		params.TxStatusDeriver,
		params.ChainID,
		params.ScheduledTransactionsEnabled,
	)
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}
	res := &execproto.GetTransactionErrorMessagesResponse_Result{
		TransactionId: lightTxResults[0].TransactionID[:],
		ErrorMessage:  expectedErrorMsg,
		Index:         1,
	}
	exeEventResp := &execproto.GetTransactionErrorMessagesResponse{
		Results: []*execproto.GetTransactionErrorMessagesResponse_Result{
			res,
		},
	}
	suite.executionAPIClient.
		On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	response, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockId, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Assert().Equal(len(lightTxResults), len(response))

	// Assertions for each transaction result in the response
	for i, responseResult := range response {
		lightTx := lightTxResults[i]
		suite.assertTransactionResultResponse(err, responseResult, *block, lightTx.TransactionID, lightTx.Failed, eventsForTx)
	}
}

func (suite *Suite) TestGetTransactionsByBlockID() {
	// Create fixtures
	col := unittest.CollectionFixture(3)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockID := block.ID()

	// Create PendingExecution events for scheduled callbacks
	pendingExecutionEvents := suite.createPendingExecutionEvents(2) // 2 callbacks

	// Reconstruct expected system collection to get the actual transaction IDs
	expectedSystemCollection, err := blueprints.SystemCollection(suite.chainID.Chain(), pendingExecutionEvents)
	suite.Require().NoError(err)

	// Expected transaction counts
	expectedUserTxCount := len(col.Transactions)
	expectedSystemTxCount := len(expectedSystemCollection.Transactions)
	expectedTotalCount := expectedUserTxCount + expectedSystemTxCount

	// Test with Local Provider
	suite.Run("LocalProvider", func() {
		// Mock the blocks storage
		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		// Mock the collections storage
		suite.collections.
			On("ByID", col.ID()).
			Return(&col, nil).
			Once()

		// Mock the events storage to return PendingExecution events
		suite.events.
			On("ByBlockID", blockID).
			Return(pendingExecutionEvents, nil).
			Once()

		suite.reporter.On("LowestIndexedHeight").Return(block.Height, nil)
		suite.reporter.On("HighestIndexedHeight").Return(block.Height+10, nil)

		// Set up the backend parameters with local transaction provider
		params := suite.defaultTransactionsParams()

		params.TxProvider = provider.NewLocalTransactionProvider(
			params.State,
			params.Collections,
			params.Blocks,
			params.EventsIndex,
			params.TxResultsIndex,
			params.TxErrorMessageProvider,
			params.SystemCollection,
			params.TxStatusDeriver,
			params.ChainID,
			params.ScheduledTransactionsEnabled,
		)

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		// Call GetTransactionsByBlockID
		transactions, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
		suite.Require().NoError(err)

		// Verify transaction count
		suite.Require().Equal(expectedTotalCount, len(transactions), "expected %d transactions but got %d", expectedTotalCount, len(transactions))

		// Verify user transactions
		for i, tx := range col.Transactions {
			suite.Assert().Equal(tx.ID(), transactions[i].ID(), "user transaction %d mismatch", i)
		}

		// Verify system transactions
		for i, expectedTx := range expectedSystemCollection.Transactions {
			actualTx := transactions[expectedUserTxCount+i]
			suite.Assert().Equal(expectedTx.ID(), actualTx.ID(), "system transaction %d mismatch", i)
		}
	})

	// Test with Execution Node Provider
	suite.Run("ExecutionNodeProvider", func() {
		// Set up the state and snapshot mocks first
		_, fixedENIDs := suite.setupReceipts(block)
		suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()
		suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
		suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
		suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil).Maybe()
		suite.snapshot.On("Head").Return(block.ToHeader(), nil).Maybe()

		// Mock the blocks storage
		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		// Mock the collections storage
		suite.collections.
			On("ByID", col.ID()).
			Return(&col, nil).
			Once()

		suite.setupExecutionGetEventsRequest(blockID, block.Height, pendingExecutionEvents)

		// Set up the backend parameters with EN transaction provider
		params := suite.defaultTransactionsParams()
		params.TxProvider = provider.NewENTransactionProvider(
			params.Log,
			params.State,
			params.Collections,
			params.ConnFactory,
			params.NodeCommunicator,
			params.NodeProvider,
			params.TxStatusDeriver,
			params.SystemCollection,
			params.ChainID,
			params.ScheduledTransactionsEnabled,
		)

		txBackend, err := NewTransactionsBackend(params)
		suite.Require().NoError(err)

		// Call GetTransactionsByBlockID
		transactions, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
		suite.Require().NoError(err)

		// For empty events, we expect: user transactions + process tx + system chunk tx (no execute callback txs)
		expectedSystemCollectionEmpty, err := blueprints.SystemCollection(suite.chainID.Chain(), pendingExecutionEvents)
		suite.Require().NoError(err)
		expectedTotalCountEmpty := len(col.Transactions) + len(expectedSystemCollectionEmpty.Transactions)

		// Verify transaction count
		suite.Assert().Equal(expectedTotalCountEmpty, len(transactions))

		// Verify user transactions
		for i, tx := range col.Transactions {
			suite.Assert().Equal(tx.ID(), transactions[i].ID())
		}

		// Verify system transactions (process + system chunk only, no execute callbacks)
		for i, expectedTx := range expectedSystemCollectionEmpty.Transactions {
			actualTx := transactions[len(col.Transactions)+i]
			suite.Assert().Equal(expectedTx.ID(), actualTx.ID())
		}
	})
}

func (suite *Suite) TestTransactionResultsByBlockIDFromExecutionNode() {
	// Create fixtures for the block and collection
	col := unittest.CollectionFixture(2)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()

	// Mock the behavior of the blocks, collections and light transaction results objects
	suite.blocks.
		On("ByID", blockId).
		Return(block, nil)
	lightCol := col.Light()
	suite.collections.
		On("LightByID", mock.Anything).
		Return(lightCol, nil).
		Once()

	// Execute callback transactions will be reconstructed from PendingExecution events
	// We don't create them manually - they'll be generated by blueprints.SystemCollection
	// System collection will be reconstructed from events, so we don't need to pre-populate lightTxResults

	lightTxResults := make([]flow.LightTransactionResult, len(lightCol.Transactions))
	for i, txID := range lightCol.Transactions {
		lightTxResults[i] = flow.LightTransactionResult{
			TransactionID:   txID,
			Failed:          false,
			ComputationUsed: 0,
		}
	}

	// Create PendingExecution events that would be emitted by the process callback transaction
	pendingExecutionEvents := suite.createPendingExecutionEvents(2) // 2 callbacks

	// Convert PendingExecution events to protobuf messages for execution node response
	pendingEventMessages := make([]*entities.Event, len(pendingExecutionEvents))
	for i, event := range pendingExecutionEvents {
		pendingEventMessages[i] = convert.EventToMessage(event)
	}

	// Reconstruct the expected system collection to get the actual transaction IDs
	expectedSystemCollection, err := blueprints.SystemCollection(suite.chainID.Chain(), pendingExecutionEvents)
	suite.Require().NoError(err)

	// Extract the expected transaction IDs in order: process, execute callbacks, system chunk
	expectedSystemTxIDs := make([]flow.Identifier, len(expectedSystemCollection.Transactions))
	for i, tx := range expectedSystemCollection.Transactions {
		expectedSystemTxIDs[i] = tx.ID()
	}

	suite.Require().Equal(4, len(expectedSystemTxIDs), "should have 4 system transactions: process + 2 execute callbacks + system chunk")

	// Build the execution response with all transaction results including proper events
	userTxResults := make([]*execproto.GetTransactionResultResponse, len(lightCol.Transactions))
	for i := 0; i < len(lightCol.Transactions); i++ {
		userTxResults[i] = &execproto.GetTransactionResultResponse{
			Events: []*entities.Event{},
		}
	}

	// System transaction results: process (with events), execute callback txs, system chunk
	systemTxResults := []*execproto.GetTransactionResultResponse{
		// Process callback transaction with PendingExecution events
		{Events: pendingEventMessages},
		// Execute callback transaction 1
		{Events: []*entities.Event{}},
		// Execute callback transaction 2
		{Events: []*entities.Event{}},
		// System chunk transaction
		{Events: []*entities.Event{}},
	}

	allTxResults := append(userTxResults, systemTxResults...)

	// Set up execution node response with system transactions
	// The execution node response should include: user txs + process tx (with PendingExecution events) + execute txs + system chunk tx
	exeGetTxReq := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockId[:],
	}
	exeGetTxResp := &execproto.GetTransactionResultsResponse{
		TransactionResults:   allTxResults,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}
	suite.executionAPIClient.
		On("GetTransactionResultsByBlockID", mock.Anything, exeGetTxReq).
		Return(exeGetTxResp, nil).
		Once()

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()
	suite.state.On("Final").Return(suite.snapshot, nil)
	suite.state.On("Sealed").Return(suite.snapshot, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head").Return(block.ToHeader(), nil)

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := suite.defaultTransactionsParams()
	params.TxErrorMessageProvider = error_messages.NewTxErrorMessageProvider(
		params.Log,
		nil,
		params.TxResultsIndex,
		params.ConnFactory,
		params.NodeCommunicator,
		params.NodeProvider,
	)

	params.TxProvider = provider.NewENTransactionProvider(
		params.Log,
		params.State,
		params.Collections,
		params.ConnFactory,
		params.NodeCommunicator,
		params.NodeProvider,
		params.TxStatusDeriver,
		params.SystemCollection,
		params.ChainID,
		params.ScheduledTransactionsEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	response, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockId, entities.EventEncodingVersion_CCF_V0)
	suite.Require().NoError(err)

	// Expected total: user transactions + system transactions (process + 2 execute callbacks + system chunk)
	expectedTotal := len(lightTxResults) + len(expectedSystemTxIDs)
	suite.Assert().Equal(expectedTotal, len(response), "should have user txs + system txs")

	// Verify user transactions
	userTxCount := len(lightCol.Transactions)
	for i := 0; i < userTxCount; i++ {
		suite.Assert().Equal(lightTxResults[i].TransactionID, response[i].TransactionID)
		suite.Assert().Equal(block.Payload.Guarantees[0].CollectionID, response[i].CollectionID)
		suite.Assert().Equal(block.ID(), response[i].BlockID)
		suite.Assert().Equal(block.Height, response[i].BlockHeight)
		suite.Assert().Equal(flow.TransactionStatusSealed, response[i].Status)
	}

	// Verify system collection transactions (all should have ZeroID as collectionID)
	systemTxCount := len(response) - userTxCount
	suite.Assert().Equal(len(expectedSystemTxIDs), systemTxCount, "should have 4 system transactions: process + 2 execute callbacks + system chunk")

	for i := 0; i < systemTxCount; i++ {
		systemTxIndex := userTxCount + i
		suite.Assert().Equal(flow.ZeroID, response[systemTxIndex].CollectionID)
		suite.Assert().Equal(block.ID(), response[systemTxIndex].BlockID)
		suite.Assert().Equal(block.Height, response[systemTxIndex].BlockHeight)
		suite.Assert().Equal(flow.TransactionStatusSealed, response[systemTxIndex].Status)
		suite.Assert().Equal(expectedSystemTxIDs[i], response[userTxCount+i].TransactionID, "system transaction %d should match reconstructed ID", i)
	}
}

// TestTransactionRetry tests that the retry mechanism will send retries at specific times
func (suite *Suite) TestTransactionRetry() {
	block := unittest.BlockFixture(
		// Height needs to be at least DefaultTransactionExpiry before we start doing retries
		unittest.Block.WithHeight(flow.DefaultTransactionExpiry + 1),
	)
	transactionBody := unittest.TransactionBodyFixture(unittest.WithReferenceBlock(block.ID()))
	headBlock := unittest.BlockFixture()
	headBlock.Height = block.Height - 1 // head is behind the current block
	suite.state.On("Final").Return(suite.snapshot, nil)

	suite.snapshot.On("Head").Return(headBlock.ToHeader(), nil)
	snapshotAtBlock := protocolmock.NewSnapshot(suite.T())
	snapshotAtBlock.On("Head").Return(block.ToHeader(), nil)
	suite.state.On("AtBlockID", block.ID()).Return(snapshotAtBlock, nil)

	// collection storage returns a not found error
	suite.collections.
		On("LightByTransactionID", transactionBody.ID()).
		Return(nil, storage.ErrNotFound)

	client := accessmock.NewAccessAPIClient(suite.T())
	params := suite.defaultTransactionsParams()
	params.StaticCollectionRPCClient = client
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	retry := retrier.NewRetrier(
		suite.log,
		suite.blocks,
		suite.collections,
		txBackend,
		txBackend.txStatusDeriver,
	)
	retry.RegisterTransaction(block.Height, &transactionBody)

	client.On("SendTransaction", mock.Anything, mock.Anything).Return(&access.SendTransactionResponse{}, nil)

	// Don't retry on every height
	err = retry.Retry(block.Height + 1)
	suite.Require().NoError(err)

	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Retry every `retryFrequency`
	err = retry.Retry(block.Height + retrier.RetryFrequency)
	suite.Require().NoError(err)

	client.AssertNumberOfCalls(suite.T(), "SendTransaction", 1)

	// do not retry if expired
	err = retry.Retry(block.Height + retrier.RetryFrequency + flow.DefaultTransactionExpiry)
	suite.Require().NoError(err)

	// Should've still only been called once
	client.AssertNumberOfCalls(suite.T(), "SendTransaction", 1)
}

// TestSuccessfulTransactionsDontRetry tests that the retry mechanism will send retries at specific times
func (suite *Suite) TestSuccessfulTransactionsDontRetry() {
	collection := unittest.CollectionFixture(1)
	light := collection.Light()
	transactionBody := collection.Transactions[0]
	txID := transactionBody.ID()

	block := unittest.BlockFixture()
	blockID := block.ID()

	// setup chain state
	_, fixedENIDs := suite.setupReceipts(block)
	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	suite.state.On("Final").Return(suite.snapshot, nil)
	suite.transactions.On("ByID", transactionBody.ID()).Return(transactionBody, nil)
	suite.collections.On("LightByTransactionID", transactionBody.ID()).Return(light, nil)
	suite.blocks.On("ByCollectionID", collection.ID()).Return(block, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	exeEventReq := execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}
	exeEventResp := execproto.GetTransactionResultResponse{
		Events: nil,
	}
	suite.executionAPIClient.
		On("GetTransactionResult", context.Background(), &exeEventReq).
		Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Times(len(fixedENIDs)) // should call each EN once

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Times(len(fixedENIDs))

	params := suite.defaultTransactionsParams()
	client := accessmock.NewAccessAPIClient(suite.T())
	params.StaticCollectionRPCClient = client
	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)

	retry := retrier.NewRetrier(
		suite.log,
		suite.blocks,
		suite.collections,
		txBackend,
		txBackend.txStatusDeriver,
	)
	retry.RegisterTransaction(block.Height, transactionBody)

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, err := txBackend.GetTransactionResult(
		context.Background(),
		txID,
		flow.ZeroID,
		flow.ZeroID,
		entities.EventEncodingVersion_JSON_CDC_V0,
	)
	suite.Require().NoError(err)
	suite.Require().NotNil(result)

	// status should be finalized since the sealed Blocks is smaller in height
	suite.Assert().Equal(flow.TransactionStatusFinalized, result.Status)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + 1)
	suite.Require().NoError(err)

	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + retrier.RetryFrequency)
	suite.Require().NoError(err)

	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + retrier.RetryFrequency + flow.DefaultTransactionExpiry)
	suite.Require().NoError(err)

	// Should've still should not be called
	client.AssertNotCalled(suite.T(), "SendTransaction", mock.Anything, mock.Anything)
}

func (suite *Suite) setupReceipts(block *flow.Block) ([]*flow.ExecutionReceipt, flow.IdentityList) {
	ids := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	receipt1 := unittest.ReceiptForBlockFixture(block)
	receipt1.ExecutorID = ids[0].NodeID
	receipt2 := unittest.ReceiptForBlockFixture(block)
	receipt2.ExecutorID = ids[1].NodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult

	receipts := flow.ExecutionReceiptList{receipt1, receipt2}
	suite.receipts.
		On("ByBlockID", block.ID()).
		Return(receipts, nil)

	return receipts, ids
}

func (suite *Suite) assertTransactionResultResponse(
	err error,
	response *accessmodel.TransactionResult,
	block flow.Block,
	txId flow.Identifier,
	txFailed bool,
	eventsForTx []flow.Event,
) {
	suite.Require().NoError(err)
	suite.Assert().Equal(block.ID(), response.BlockID)
	suite.Assert().Equal(block.Height, response.BlockHeight)
	suite.Assert().Equal(txId, response.TransactionID)
	if txId == suite.defaultSystemCollection.SystemTxID() {
		suite.Assert().Equal(flow.ZeroID, response.CollectionID)
	} else {
		suite.Assert().Equal(block.Payload.Guarantees[0].CollectionID, response.CollectionID)
	}
	suite.Assert().Equal(len(eventsForTx), len(response.Events))
	// When there are error messages occurred in the transaction, the status should be 1
	if txFailed {
		suite.Assert().Equal(uint(1), response.StatusCode)
		suite.Assert().Equal(expectedErrorMsg, response.ErrorMessage)
	} else {
		suite.Assert().Equal(uint(0), response.StatusCode)
		suite.Assert().Equal("", response.ErrorMessage)
	}
	suite.Assert().Equal(flow.TransactionStatusSealed, response.Status)
}

// createPendingExecutionEvents creates properly formatted PendingExecution events
// that blueprints.SystemCollection expects for reconstructing the system collection.
func (suite *Suite) createPendingExecutionEvents(numCallbacks int) []flow.Event {
	events := make([]flow.Event, numCallbacks)

	// Get system contracts for the test chain
	env := systemcontracts.SystemContractsForChain(suite.chainID).AsTemplateEnv()

	for i := 0; i < numCallbacks; i++ {
		// Create the PendingExecution event as it would be emitted by the process callback transaction
		const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"
		eventTypeString := fmt.Sprintf(processedEventTypeTemplate, env.FlowTransactionSchedulerAddress)

		// Create Cadence event type
		loc, err := cadenceCommon.HexToAddress(env.FlowTransactionSchedulerAddress)
		suite.Require().NoError(err)
		location := cadenceCommon.NewAddressLocation(nil, loc, "PendingExecution")

		eventType := cadence.NewEventType(
			location,
			"PendingExecution",
			[]cadence.Field{
				{Identifier: "id", Type: cadence.UInt64Type},
				{Identifier: "priority", Type: cadence.UInt8Type},
				{Identifier: "executionEffort", Type: cadence.UInt64Type},
				{Identifier: "fees", Type: cadence.UFix64Type},
				{Identifier: "callbackOwner", Type: cadence.AddressType},
			},
			nil,
		)

		fees, err := cadence.NewUFix64("0.0")
		suite.Require().NoError(err)

		// Create the Cadence event with proper values
		event := cadence.NewEvent(
			[]cadence.Value{
				cadence.NewUInt64(uint64(i + 1)),           // id: unique callback ID
				cadence.NewUInt8(1),                        // priority
				cadence.NewUInt64(uint64((i+1)*100 + 100)), // executionEffort (200, 300, etc.)
				fees,                          // fees: 0.0
				cadence.NewAddress([8]byte{}), // callbackOwner
			},
		).WithType(eventType)

		// Encode the event using CCF
		payload, err := ccf.Encode(event)
		suite.Require().NoError(err)

		// Create the Flow event
		events[i] = flow.Event{
			Type:             flow.EventType(eventTypeString),
			TransactionID:    unittest.IdentifierFixture(), // Process callback transaction ID
			TransactionIndex: 0,
			EventIndex:       uint32(i),
			Payload:          payload,
		}
	}

	return events
}

func (suite *Suite) setupExecutionGetEventsRequest(blockID flow.Identifier, blockHeight uint64, events []flow.Event) {
	eventMessages := make([]*entities.Event, len(events))
	for i, event := range events {
		eventMessages[i] = convert.EventToMessage(event)
	}

	request := &execproto.GetEventsForBlockIDsRequest{
		Type:     string(suite.processScheduledTransactionEventType),
		BlockIds: [][]byte{blockID[:]},
	}
	expectedResponse := &execproto.GetEventsForBlockIDsResponse{
		Results: []*execproto.GetEventsForBlockIDsResponse_Result{
			{
				BlockId:     blockID[:],
				BlockHeight: blockHeight,
				Events:      eventMessages,
			},
		},
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}

	suite.executionAPIClient.
		On("GetEventsForBlockIDs", mock.Anything, request).
		Return(expectedResponse, nil).
		Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()
}
