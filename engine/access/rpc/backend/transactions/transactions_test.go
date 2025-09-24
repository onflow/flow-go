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
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/retrier"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	optimisticsyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/metrics"
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

	chainID                           flow.ChainID
	systemTx                          *flow.TransactionBody
	systemCollection                  *flow.Collection
	pendingExecutionEvents            []flow.Event
	processScheduledCallbackEventType flow.EventType
	scheduledCallbacksEnabled         bool

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList

	execResultProvider  *optimisticsyncmock.ExecutionResultInfoProvider
	executionStateCache *optimisticsyncmock.ExecutionStateCache
	operatorCriteria    optimistic_sync.Criteria

	rootBlock       *flow.Block
	rootBlockHeader *flow.Header
	receiptsList    flow.ExecutionReceiptList
	identities      flow.IdentityList
}

func TestTransactionsBackend(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (s *Suite) SetupTest() {
	s.log = unittest.Logger()
	s.snapshot = protocolmock.NewSnapshot(s.T())

	s.rootBlock = unittest.BlockFixture()
	s.rootBlockHeader = s.rootBlock.ToHeader()
	s.params = protocolmock.NewParams(s.T())
	s.params.On("FinalizedRoot").Return(s.rootBlockHeader, nil).Maybe()
	s.params.On("SporkID").Return(unittest.IdentifierFixture(), nil).Maybe()
	s.params.On("SporkRootBlockHeight").Return(s.rootBlockHeader.Height, nil).Maybe()
	s.params.On("SealedRoot").Return(s.rootBlockHeader, nil).Maybe()

	s.state = protocolmock.NewState(s.T())
	s.state.On("Params").Return(s.params).Maybe()

	s.blocks = storagemock.NewBlocks(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.transactions = storagemock.NewTransactions(s.T())
	s.collections = storagemock.NewCollections(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.results = storagemock.NewExecutionResults(s.T())
	s.txResultErrorMessages = storagemock.NewTransactionResultErrorMessages(s.T())
	s.executionAPIClient = accessmock.NewExecutionAPIClient(s.T())
	s.lightTxResults = storagemock.NewLightTransactionResults(s.T())
	s.events = storagemock.NewEvents(s.T())
	s.chainID = flow.Testnet
	s.historicalAccessAPIClient = accessmock.NewAccessAPIClient(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())

	txResCache, err := lru.New[flow.Identifier, *accessmodel.TransactionResult](10)
	s.Require().NoError(err)
	s.txResultCache = txResCache

	s.systemTx, err = blueprints.SystemChunkTransaction(flow.Testnet.Chain())
	s.Require().NoError(err)

	s.systemTx, err = blueprints.SystemChunkTransaction(flow.Testnet.Chain())
	s.Require().NoError(err)
	s.scheduledCallbacksEnabled = true

	s.pendingExecutionEvents = s.createPendingExecutionEvents(2) // 2 callbacks
	s.systemCollection, err = blueprints.SystemCollection(s.chainID.Chain(), s.pendingExecutionEvents)
	s.Require().NoError(err)
	s.processScheduledCallbackEventType = s.pendingExecutionEvents[0].Type

	s.db, s.dbDir = unittest.TempPebbleDB(s.T())
	progress, err := store.NewConsumerProgress(pebbleimpl.ToDB(s.db), module.ConsumeProgressLastFullBlockHeight).Initialize(0)
	s.Require().NoError(err)
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	s.Require().NoError(err)

	s.execResultProvider = optimisticsyncmock.NewExecutionResultInfoProvider(s.T())
	s.executionStateCache = optimisticsyncmock.NewExecutionStateCache(s.T())
	s.operatorCriteria = optimistic_sync.Criteria{}

	s.fixedExecutionNodeIDs = nil
	s.preferredExecutionNodeIDs = nil

	s.identities = unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

	receipt1 := unittest.ReceiptForBlockFixture(s.rootBlock)
	receipt1.ExecutorID = s.identities[0].NodeID
	receipt2 := unittest.ReceiptForBlockFixture(s.rootBlock)
	receipt2.ExecutorID = s.identities[1].NodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult

	s.receiptsList = flow.ExecutionReceiptList{receipt1, receipt2}
}

func (s *Suite) TearDownTest() {
	err := os.RemoveAll(s.dbDir)
	s.Require().NoError(err)
}

func (s *Suite) defaultTransactionsParams() Params {
	nodeProvider := rpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.state,
		s.receipts,
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodeIDs,
	)

	txStatusDeriver := txstatus.NewTxStatusDeriver(
		s.state,
		s.lastFullBlockHeight,
	)

	txValidator, err := validator.NewTransactionValidator(
		validatormock.NewBlocks(s.T()),
		s.chainID.Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{},
		execmock.NewScriptExecutor(s.T()),
	)
	s.Require().NoError(err)

	nodeCommunicator := node_communicator.NewNodeCommunicator(false)

	txProvider := provider.NewENTransactionProvider(
		s.log,
		s.state,
		s.collections,
		s.connectionFactory,
		nodeCommunicator,
		txStatusDeriver,
		s.systemTx.ID(),
		s.chainID,
		s.scheduledCallbacksEnabled,
	)

	return Params{
		Log:                         s.log,
		Metrics:                     metrics.NewNoopCollector(),
		State:                       s.state,
		ChainID:                     flow.Testnet,
		SystemTxID:                  s.systemTx.ID(),
		StaticCollectionRPCClient:   s.historicalAccessAPIClient,
		HistoricalAccessNodeClients: nil,
		NodeCommunicator:            nodeCommunicator,
		ConnFactory:                 s.connectionFactory,
		EnableRetries:               true,
		NodeProvider:                nodeProvider,
		Blocks:                      s.blocks,
		Collections:                 s.collections,
		Transactions:                s.transactions,
		TxResultCache:               s.txResultCache,
		TxProvider:                  txProvider,
		TxValidator:                 txValidator,
		TxStatusDeriver:             txStatusDeriver,
		ExecutionStateCache:         s.executionStateCache,
		ExecResultProvider:          s.execResultProvider,
		OperatorCriteria:            s.operatorCriteria,
		ScheduledCallbacksEnabled:   s.scheduledCallbacksEnabled,
	}
}

// TestGetTransactionResult_UnknownTx returns unknown result when tx not found
func (s *Suite) TestGetTransactionResult_UnknownTx() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

	s.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	params := s.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)
	res, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(res.Status, flow.TransactionStatusUnknown)
	s.Require().Empty(res.BlockID)
	s.Require().Empty(res.BlockHeight)
	s.Require().Empty(res.TransactionID)
	s.Require().Empty(res.CollectionID)
	s.Require().Empty(res.ErrorMessage)
}

// TestGetTransactionResult_TxLookupFailure returns error from transaction storage
func (s *Suite) TestGetTransactionResult_TxLookupFailure() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

	expectedErr := fmt.Errorf("some other error")
	s.transactions.
		On("ByID", tx.ID()).
		Return(nil, expectedErr)

	params := s.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)

	_, _, err = txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().Equal(err, status.Errorf(codes.Internal, "failed to find: %v", expectedErr))
}

// TestGetTransactionResult_HistoricNodes_Success tests lookup in historic nodes
func (s *Suite) TestGetTransactionResult_HistoricNodes_Success() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

	s.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	transactionResultResponse := access.TransactionResultResponse{
		Status:     entities.TransactionStatus_EXECUTED,
		StatusCode: uint32(entities.TransactionStatus_EXECUTED),
	}

	s.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(&transactionResultResponse, nil).
		Once()

	params := s.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{s.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)

	resp, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
	s.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)
}

// TestGetTransactionResult_HistoricNodes_FromCache get historic transaction result from cache
func (s *Suite) TestGetTransactionResult_HistoricNodes_FromCache() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody

	s.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	transactionResultResponse := access.TransactionResultResponse{
		Status:     entities.TransactionStatus_EXECUTED,
		StatusCode: uint32(entities.TransactionStatus_EXECUTED),
	}

	s.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(&transactionResultResponse, nil).
		Once()

	params := s.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{s.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)

	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})
	resp, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
	s.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)

	resp2, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(flow.TransactionStatusExecuted, resp2.Status)
	s.Require().Equal(uint(flow.TransactionStatusExecuted), resp2.StatusCode)
}

// TestGetTransactionResultUnknownFromCache retrieve unknown result from cache.
func (s *Suite) TestGetTransactionResultUnknownFromCache() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody

	s.transactions.
		On("ByID", tx.ID()).
		Return(nil, storage.ErrNotFound)

	s.historicalAccessAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *access.GetTransactionRequest) bool {
			txID := tx.ID()
			return bytes.Equal(txID[:], req.Id)
		})).
		Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).
		Once()

	params := s.defaultTransactionsParams()
	params.HistoricalAccessNodeClients = []access.AccessAPIClient{s.historicalAccessAPIClient}
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)

	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})
	resp, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(flow.TransactionStatusUnknown, resp.Status)
	s.Require().Equal(uint(flow.TransactionStatusUnknown), resp.StatusCode)

	// ensure the unknown transaction is cached when not found anywhere
	txStatus := flow.TransactionStatusUnknown
	res, ok := txBackend.txResultCache.Get(tx.ID())
	s.Require().True(ok)
	s.Require().Equal(res, &accessmodel.TransactionResult{
		Status:     txStatus,
		StatusCode: uint(txStatus),
	})

	// ensure underlying GetTransactionResult() won't be called the second time
	resp2, _, err := txBackend.GetTransactionResult(
		context.Background(),
		tx.ID(),
		block.ID(),
		coll.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().Equal(flow.TransactionStatusUnknown, resp2.Status)
	s.Require().Equal(uint(flow.TransactionStatusUnknown), resp2.StatusCode)
}

// TestGetSystemTransaction_HappyPath tests that GetSystemTransaction call returns system chunk transaction.
func (s *Suite) TestGetSystemTransaction_ExecutionNode_HappyPath() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	params := s.defaultTransactionsParams()
	enabledProvider := provider.NewENTransactionProvider(
		s.log,
		s.state,
		s.collections,
		s.connectionFactory,
		params.NodeCommunicator,
		params.TxStatusDeriver,
		s.systemTx.ID(),
		s.chainID,
		true,
	)
	disabledProvider := provider.NewENTransactionProvider(
		s.log,
		s.state,
		s.collections,
		s.connectionFactory,
		params.NodeCommunicator,
		params.TxStatusDeriver,
		s.systemTx.ID(),
		s.chainID,
		false,
	)

	s.params.On("FinalizedRoot").Unset()

	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: s.receiptsList[0].ExecutionResult.ID(),
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)

	s.Run("scheduled callbacks DISABLED - ZeroID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks DISABLED - system txID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), s.systemTx.ID(), blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks DISABLED - non-system txID fails", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID, optimistic_sync.Criteria{})

		s.Require().Error(err)
		s.Require().Nil(res)
	})

	s.Run("scheduled callbacks ENABLED - ZeroID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - system txID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), s.systemTx.ID(), blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - system collection TX", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		s.setupExecutionGetEventsRequest(blockID, block.Height, s.pendingExecutionEvents)

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		systemTx := s.systemCollection.Transactions[2]
		res, _, err := txBackend.GetSystemTransaction(context.Background(), systemTx.ID(), blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - non-system txID fails", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		s.setupExecutionGetEventsRequest(blockID, block.Height, s.pendingExecutionEvents)

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID, optimistic_sync.Criteria{})

		s.Require().Error(err)
		s.Require().Nil(res)
	})
}

// TestGetSystemTransaction_HappyPath tests that GetSystemTransaction call returns system chunk transaction.
func (s *Suite) TestGetSystemTransaction_Local_HappyPath() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	executionResult := s.receiptsList[0].ExecutionResult
	executionResultID := executionResult.ID()

	params := s.defaultTransactionsParams()
	enabledProvider := provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.systemTx.ID(),
		params.TxStatusDeriver,
		params.ExecutionStateCache,
		s.chainID,
		true,
	)
	disabledProvider := provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.systemTx.ID(),
		params.TxStatusDeriver,
		params.ExecutionStateCache,
		s.chainID,
		false,
	)

	s.params.On("FinalizedRoot").Unset()

	// this is called for all tests
	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: executionResultID,
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)

	s.Run("scheduled callbacks DISABLED - ZeroID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks DISABLED - system txID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), s.systemTx.ID(), blockID, optimistic_sync.Criteria{})
		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks DISABLED - non-system txID fails", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = disabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID, optimistic_sync.Criteria{})

		s.Require().Error(err)
		s.Require().Nil(res)
	})

	s.Run("scheduled callbacks ENABLED - ZeroID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), flow.ZeroID, blockID, optimistic_sync.Criteria{})

		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - system txID", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()

		params.TxProvider = enabledProvider

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), s.systemTx.ID(), blockID, optimistic_sync.Criteria{})

		s.Require().NoError(err)

		s.Require().Equal(s.systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - system collection TX", func() {
		systemTx := s.systemCollection.Transactions[2]
		systemTxID := systemTx.ID()
		params.TxProvider = enabledProvider

		s.blocks.On("ByID", blockID).Return(block, nil).Once()
		s.events.On("ByBlockID", blockID).Return(s.pendingExecutionEvents, nil).Once()

		snapshot := optimisticsyncmock.NewSnapshot(s.T())
		snapshot.On("Events").Return(s.events)

		s.executionStateCache.
			On("Snapshot", executionResultID).
			Return(snapshot, nil).
			Once()

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), systemTxID, blockID, optimistic_sync.Criteria{})

		s.Require().NoError(err)
		s.Require().Equal(systemTx, res)
	})

	s.Run("scheduled callbacks ENABLED - non-system txID fails", func() {
		params.TxProvider = enabledProvider

		s.blocks.On("ByID", blockID).Return(block, nil).Once()
		s.events.On("ByBlockID", blockID).Return(s.pendingExecutionEvents, nil).Once()

		snapshot := optimisticsyncmock.NewSnapshot(s.T())
		snapshot.On("Events").Return(s.events)

		s.executionStateCache.
			On("Snapshot", executionResultID).
			Return(snapshot, nil).
			Once()

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		res, _, err := txBackend.GetSystemTransaction(context.Background(), unittest.IdentifierFixture(), blockID, optimistic_sync.Criteria{})

		s.Require().Error(err)
		s.Require().Nil(res)
	})
}

func (s *Suite) TestGetSystemTransactionResult_ExecutionNode_HappyPath() {
	test := func(snapshot protocol.Snapshot) {
		s.state.
			On("Sealed").
			Return(snapshot, nil).
			Once()

		lastBlock, err := snapshot.Head()
		s.Require().NoError(err)

		identities, err := snapshot.Identities(filter.Any)
		s.Require().NoError(err)

		block := unittest.BlockWithParentFixture(lastBlock)
		blockID := block.ID()

		stateSnapshotForKnownBlock := unittest.StateSnapshotForKnownBlock(block.ToHeader(), identities.Lookup())

		// block storage returns the corresponding block
		s.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		stateSnapshotForKnownBlock.On("SealedResult").Return(&s.receiptsList[0].ExecutionResult, nil, nil)

		// Generating events with event generator
		exeNodeEventEncodingVersion := entities.EventEncodingVersion_CCF_V0
		events := unittest.EventGenerator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
		eventMessages := convert.EventsToMessages(events)

		systemTxID := s.systemTx.ID()
		expectedRequest := &execproto.GetTransactionResultRequest{
			BlockId:       blockID[:],
			TransactionId: systemTxID[:],
		}
		exeEventResp := &execproto.GetTransactionResultResponse{
			Events:               eventMessages,
			EventEncodingVersion: exeNodeEventEncodingVersion,
		}

		s.executionAPIClient.
			On("GetTransactionResult", mock.Anything, expectedRequest).
			Return(exeEventResp, nil).
			Once()

		s.connectionFactory.
			On("GetExecutionAPIClient", mock.Anything).
			Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
			Once()

		// the connection factory should be used to get the execution node client
		params := s.defaultTransactionsParams()
		backend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		s.execResultProvider.
			On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResultID: s.receiptsList[0].ExecutionResult.ID(),
				ExecutionNodes:    s.identities.ToSkeleton(),
			}, nil)

		res, _, err := backend.GetSystemTransactionResult(
			context.Background(),
			flow.ZeroID,
			block.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
			optimistic_sync.Criteria{},
		)
		s.Require().NoError(err)

		// Expected system chunk transaction
		s.Require().Equal(flow.TransactionStatusExecuted, res.Status)
		s.Require().Equal(s.systemTx.ID(), res.TransactionID)

		// Check for successful decoding of event
		_, err = jsoncdc.Decode(nil, res.Events[0].Payload)
		s.Require().NoError(err)

		events, err = convert.MessagesToEventsWithEncodingConversion(
			eventMessages,
			exeNodeEventEncodingVersion,
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		s.Require().NoError(err)
		s.Require().Equal(events, res.Events)
	}

	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(
		s.T(),
		rootSnapshot,
		func(db storage.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
			epochBuilder := unittest.NewEpochBuilder(s.T(), mutableState, state)

			epochBuilder.
				BuildEpoch().
				CompleteEpoch()

			// get heights of each phase in built epochs
			epoch1, ok := epochBuilder.EpochHeights(1)
			require.True(s.T(), ok)

			snapshot := state.AtHeight(epoch1.FinalHeight())
			test(snapshot)
		},
	)
}

func (s *Suite) TestGetSystemTransactionResult_Local_HappyPath() {
	block := unittest.BlockFixture()
	sysTx, err := blueprints.SystemChunkTransaction(s.chainID.Chain())
	s.Require().NoError(err)
	s.Require().NotNil(sysTx)
	txId := s.systemTx.ID()
	blockId := block.ID()

	reqCriteria := optimistic_sync.Criteria{
		AgreeingExecutorsCount: 3,
		RequiredExecutors:      flow.IdentifierList{unittest.IdentifierFixture()},
	}

	s.blocks.
		On("ByID", blockId).
		Return(block, nil).
		Once()

	lightResult := &flow.LightTransactionResult{
		TransactionID:   txId,
		Failed:          false,
		ComputationUsed: 0,
	}
	s.lightTxResults.
		On("ByBlockIDTransactionID", blockId, txId).
		Return(lightResult, nil).
		Once()

	// Set up the events storage mock
	var eventsForTx []flow.Event
	// expect a call to lookup events by block ID and transaction ID
	s.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the backend parameters and the backend instance
	params := s.defaultTransactionsParams()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResultInfo", blockId, reqCriteria).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResultID: fakeRes.ID()}, nil)
	s.executionStateCache.
		On("Snapshot", fakeRes.ID()).
		Return(snapshot, nil)

	// Snapshot readers delegate to our storage mocks
	snapshot.On("Events").Return(s.events)
	snapshot.On("LightTransactionResults").Return(s.lightTxResults)

	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.SystemTxID,
		params.TxStatusDeriver,
		s.executionStateCache,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)
	response, metadata, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		flow.ZeroID,
		blockId,
		entities.EventEncodingVersion_JSON_CDC_V0,
		reqCriteria,
	)

	_ = metadata // TODO: validate metadata

	s.assertTransactionResultResponse(err, response, *block, txId, lightResult.Failed, eventsForTx)
}

// TestGetSystemTransactionResult_BlockNotFound tests GetSystemTransactionResult function when block was not found.
func (s *Suite) TestGetSystemTransactionResult_BlockNotFound() {
	block := unittest.BlockFixture()
	s.blocks.
		On("ByID", block.ID()).
		Return(nil, storage.ErrNotFound).
		Once()

	params := s.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)
	res, _, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		flow.ZeroID,
		block.ID(),
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)

	s.Require().Nil(res)
	s.Require().Error(err)
	s.Require().Equal(err, status.Errorf(codes.NotFound, "not found: %v", fmt.Errorf("key not found")))
}

// TestGetSystemTransactionResult_FailedEncodingConversion tests the GetSystemTransactionResult function with different
// event encoding versions.
func (s *Suite) TestGetSystemTransactionResult_FailedEncodingConversion() {
	block := unittest.BlockFixture()
	blockID := block.ID()

	s.state.On("Sealed").Return(s.snapshot, nil)
	s.snapshot.On("Head").Return(block.ToHeader(), nil)

	// block storage returns the corresponding block
	s.blocks.
		On("ByID", blockID).
		Return(block, nil).
		Once()

	// create empty events
	eventsPerBlock := 10
	eventMessages := make([]*entities.Event, eventsPerBlock)

	systemTxID := s.systemTx.ID()
	expectedRequest := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: systemTxID[:],
	}
	exeEventResp := &execproto.GetTransactionResultResponse{
		Events:               eventMessages,
		EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
	}

	s.executionAPIClient.
		On("GetTransactionResult", mock.Anything, expectedRequest).
		Return(exeEventResp, nil).
		Once()

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := s.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: s.receiptsList[0].ExecutionResult.ID(),
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)

	res, _, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		flow.ZeroID,
		block.ID(),
		entities.EventEncodingVersion_CCF_V0,
		optimistic_sync.Criteria{},
	)

	s.Require().Nil(res)
	s.Require().Error(err)
	s.Require().Equal(err, status.Errorf(codes.Internal, "failed to convert events to message: %v",
		fmt.Errorf("conversion from format JSON_CDC_V0 to CCF_V0 is not supported")))
}

// TestGetTransactionResult_FromStorage tests the retrieval of a transaction result (flow.TransactionResult) from storage
// instead of requesting it from the Execution Node.
func (s *Suite) TestGetTransactionResult_FromStorage() {
	// Create fixtures for block, transaction, and collection
	transaction := unittest.TransactionFixture()
	col := unittest.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	txId := transaction.ID()
	blockId := block.ID()

	s.blocks.
		On("ByID", blockId).
		Return(block, nil)

	s.lightTxResults.On("ByBlockIDTransactionID", blockId, txId).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil)

	s.transactions.
		On("ByID", txId).
		Return(&transaction.TransactionBody, nil)

	// Set up the light collection and mock the behavior of the collections object
	lightCol := col.Light()
	s.collections.On("LightByID", col.ID()).Return(lightCol, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}
	// expect a call to lookup events by block ID and transaction ID
	s.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	s.txResultErrorMessages.On("ByBlockIDTransactionID", mock.Anything, mock.Anything).Return(
		&flow.TransactionResultErrorMessage{
			TransactionID: txId,
			ErrorMessage:  expectedErrorMsg,
			Index:         0,
			ExecutorID:    unittest.IdentifierFixture(),
		}, nil)

	params := s.defaultTransactionsParams()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)
	snapshot.On("Events").Return(s.events)
	snapshot.On("LightTransactionResults").Return(s.lightTxResults)
	snapshot.On("TransactionResultErrorMessages").Return(s.txResultErrorMessages)
	snapshot.On("Collections").Return(s.collections)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: fakeRes.ID(),
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)
	s.executionStateCache.On("Snapshot", mock.Anything).Return(snapshot, nil)

	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.SystemTxID,
		params.TxStatusDeriver,
		s.executionStateCache,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	response, _, err := txBackend.GetTransactionResult(context.Background(), txId, blockId, flow.ZeroID,
		entities.EventEncodingVersion_JSON_CDC_V0, optimistic_sync.Criteria{})
	s.assertTransactionResultResponse(err, response, *block, txId, true, eventsForTx)

	s.blocks.AssertExpectations(s.T())
	s.events.AssertExpectations(s.T())
	s.state.AssertExpectations(s.T())
}

// TestTransactionByIndexFromStorage tests the retrieval of a transaction result (flow.TransactionResult) by index
// and returns it from storage instead of requesting from the Execution Node.
func (s *Suite) TestTransactionByIndexFromStorage() {
	// Create fixtures for block, transaction, and collection
	transaction := unittest.TransactionFixture()
	col := unittest.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()
	txId := transaction.ID()
	txIndex := rand.Uint32()

	// Set up the light collection and mock the behavior of the collections object
	lightCol := col.Light()
	s.collections.On("LightByID", col.ID()).Return(lightCol, nil)

	// Mock the behavior of the blocks and lightTxResults objects
	s.blocks.
		On("ByID", blockId).
		Return(block, nil)

	s.lightTxResults.On("ByBlockIDTransactionIndex", blockId, txIndex).
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
	s.events.On("ByBlockIDTransactionIndex", blockId, txIndex).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	s.txResultErrorMessages.On("ByBlockIDTransactionIndex", mock.Anything, mock.Anything).Return(
		&flow.TransactionResultErrorMessage{
			TransactionID: txId,
			ErrorMessage:  expectedErrorMsg,
			Index:         0,
			ExecutorID:    unittest.IdentifierFixture(),
		}, nil)

	params := s.defaultTransactionsParams()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResultID: fakeRes.ID()}, nil)
	s.executionStateCache.On("Snapshot", mock.Anything).Return(snapshot, nil)

	// Snapshot readers delegate to our storage mocks
	snapshot.On("Events").Return(s.events)
	snapshot.On("LightTransactionResults").Return(s.lightTxResults)
	snapshot.On("Collections").Return(s.collections)
	snapshot.On("TransactionResultErrorMessages").Return(s.txResultErrorMessages)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.SystemTxID,
		params.TxStatusDeriver,
		s.executionStateCache,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	response, _, err := txBackend.GetTransactionResultByIndex(context.Background(), blockId, txIndex,
		entities.EventEncodingVersion_JSON_CDC_V0, optimistic_sync.Criteria{})
	s.assertTransactionResultResponse(err, response, *block, txId, true, eventsForTx)
}

// TestTransactionResultsByBlockIDFromStorage tests the retrieval of transaction results ([]flow.TransactionResult)
// by block ID from storage instead of requesting from the Execution Node.
func (s *Suite) TestTransactionResultsByBlockIDFromStorage() {
	// Create fixtures for the block and collection
	col := unittest.CollectionFixture(2)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()

	// Mock the behavior of the blocks, collections and light transaction results objects
	s.blocks.
		On("ByID", blockId).
		Return(block, nil)
	lightCol := col.Light()
	s.collections.
		On("LightByID", mock.Anything).
		Return(lightCol, nil).
		Once()

	lightTxResults := make([]flow.LightTransactionResult, len(lightCol.Transactions))
	txErrorMessageList := make([]flow.TransactionResultErrorMessage, 0)
	for i, txID := range lightCol.Transactions {
		lightTxResults[i] = flow.LightTransactionResult{
			TransactionID:   txID,
			Failed:          i == 0, // Mark the first transaction as failed
			ComputationUsed: 0,
		}
		if lightTxResults[i].Failed {
			txErrorMessageList = append(txErrorMessageList, flow.TransactionResultErrorMessage{
				TransactionID: txID,
				ErrorMessage:  expectedErrorMsg,
				Index:         uint32(i),
				ExecutorID:    unittest.IdentifierFixture(),
			})
		}
	}
	// simulate the system tx
	lightTxResults = append(lightTxResults, flow.LightTransactionResult{
		TransactionID:   s.systemTx.ID(),
		Failed:          false,
		ComputationUsed: 10,
	})

	s.lightTxResults.
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
	s.events.
		On("ByBlockIDTransactionID", blockId, mock.Anything).
		Return(eventsForTx, nil)

	s.txResultErrorMessages.
		On("ByBlockID", blockId).
		Return(txErrorMessageList, nil)

	// Set up the state and snapshot mocks

	params := s.defaultTransactionsParams()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResultID: fakeRes.ID()}, nil)
	s.executionStateCache.On("Snapshot", mock.Anything).Return(snapshot, nil)

	// Snapshot readers delegate to our storage mocks
	snapshot.On("Events").Return(s.events)
	snapshot.On("LightTransactionResults").Return(s.lightTxResults)
	snapshot.On("Collections").Return(s.collections)
	snapshot.On("TransactionResultErrorMessages").Return(s.txResultErrorMessages)
	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.SystemTxID,
		params.TxStatusDeriver,
		s.executionStateCache,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	response, _, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockId,
		entities.EventEncodingVersion_JSON_CDC_V0, optimistic_sync.Criteria{})
	s.Require().NoError(err)
	s.Assert().Equal(len(lightTxResults), len(response))

	// Assertions for each transaction result in the response
	for i, responseResult := range response {
		lightTx := lightTxResults[i]
		s.assertTransactionResultResponse(err, responseResult, *block, lightTx.TransactionID, lightTx.Failed, eventsForTx)
	}
}

func (s *Suite) TestGetTransactionsByBlockID() {
	// Create fixtures
	col := unittest.CollectionFixture(3)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockID := block.ID()

	// Create PendingExecution events for scheduled callbacks
	pendingExecutionEvents := s.createPendingExecutionEvents(2) // 2 callbacks

	// Reconstruct expected system collection to get the actual transaction IDs
	expectedSystemCollection, err := blueprints.SystemCollection(s.chainID.Chain(), pendingExecutionEvents)
	s.Require().NoError(err)

	// Expected transaction counts
	expectedUserTxCount := len(col.Transactions)
	expectedSystemTxCount := len(expectedSystemCollection.Transactions)
	expectedTotalCount := expectedUserTxCount + expectedSystemTxCount

	executionResult := s.receiptsList[0].ExecutionResult
	executionResultID := executionResult.ID()

	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: executionResultID,
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)

	// Test with Local Provider
	s.Run("LocalProvider", func() {
		snapshot := optimisticsyncmock.NewSnapshot(s.T())
		snapshot.On("Events").Return(s.events)
		snapshot.On("Collections").Return(s.collections)

		s.executionStateCache.
			On("Snapshot", executionResultID).
			Return(snapshot, nil).
			Once()

		s.blocks.On("ByID", blockID).Return(block, nil).Once()
		s.collections.On("ByID", col.ID()).Return(&col, nil).Once()
		s.events.On("ByBlockID", blockID).Return(pendingExecutionEvents, nil).Once()

		// Set up the backend parameters with local transaction provider
		params := s.defaultTransactionsParams()

		params.TxProvider = provider.NewLocalTransactionProvider(
			params.State,
			params.Collections,
			params.Blocks,
			params.SystemTxID,
			params.TxStatusDeriver,
			params.ExecutionStateCache,
			params.ChainID,
			params.ScheduledCallbacksEnabled,
		)

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		// Call GetTransactionsByBlockID
		transactions, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
		s.Require().NoError(err)

		// Verify transaction count
		s.Require().Equal(expectedTotalCount, len(transactions), "expected %d transactions but got %d", expectedTotalCount, len(transactions))

		// Verify user transactions
		for i, tx := range col.Transactions {
			s.Assert().Equal(tx.ID(), transactions[i].ID(), "user transaction %d mismatch", i)
		}

		// Verify system transactions
		for i, expectedTx := range expectedSystemCollection.Transactions {
			actualTx := transactions[expectedUserTxCount+i]
			s.Assert().Equal(expectedTx.ID(), actualTx.ID(), "system transaction %d mismatch", i)
		}
	})

	// Test with Execution Node Provider
	s.Run("ExecutionNodeProvider", func() {
		s.blocks.On("ByID", blockID).Return(block, nil).Once()
		s.collections.On("ByID", col.ID()).Return(&col, nil).Once()

		s.setupExecutionGetEventsRequest(blockID, block.Height, pendingExecutionEvents)

		// Set up the backend parameters with EN transaction provider
		params := s.defaultTransactionsParams()
		params.TxProvider = provider.NewENTransactionProvider(
			params.Log,
			params.State,
			params.Collections,
			params.ConnFactory,
			params.NodeCommunicator,
			params.TxStatusDeriver,
			params.SystemTxID,
			params.ChainID,
			params.ScheduledCallbacksEnabled,
		)

		txBackend, err := NewTransactionsBackend(params)
		s.Require().NoError(err)

		// Call GetTransactionsByBlockID
		transactions, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
		s.Require().NoError(err)

		// For empty events, we expect: user transactions + process tx + system chunk tx (no execute callback txs)
		expectedSystemCollectionEmpty, err := blueprints.SystemCollection(s.chainID.Chain(), pendingExecutionEvents)
		s.Require().NoError(err)
		expectedTotalCountEmpty := len(col.Transactions) + len(expectedSystemCollectionEmpty.Transactions)

		// Verify transaction count
		s.Assert().Equal(expectedTotalCountEmpty, len(transactions))

		// Verify user transactions
		for i, tx := range col.Transactions {
			s.Assert().Equal(tx.ID(), transactions[i].ID())
		}

		// Verify system transactions (process + system chunk only, no execute callbacks)
		for i, expectedTx := range expectedSystemCollectionEmpty.Transactions {
			actualTx := transactions[len(col.Transactions)+i]
			s.Assert().Equal(expectedTx.ID(), actualTx.ID())
		}
	})
}

func (s *Suite) TestTransactionResultsByBlockIDFromExecutionNode() {
	// Create fixtures for the block and collection
	col := unittest.CollectionFixture(2)
	guarantee := &flow.CollectionGuarantee{CollectionID: col.ID()}
	block := unittest.BlockFixture(
		unittest.Block.WithPayload(unittest.PayloadFixture(unittest.WithGuarantees(guarantee))),
	)
	blockId := block.ID()

	// Mock the behavior of the blocks, collections and light transaction results objects
	lightCol := col.Light()
	s.blocks.On("ByID", blockId).Return(block, nil)
	s.collections.On("LightByID", mock.Anything).Return(lightCol, nil).Once()

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
	pendingExecutionEvents := s.createPendingExecutionEvents(2) // 2 callbacks

	// Convert PendingExecution events to protobuf messages for execution node response
	pendingEventMessages := make([]*entities.Event, len(pendingExecutionEvents))
	for i, event := range pendingExecutionEvents {
		pendingEventMessages[i] = convert.EventToMessage(event)
	}

	// Reconstruct the expected system collection to get the actual transaction IDs
	expectedSystemCollection, err := blueprints.SystemCollection(s.chainID.Chain(), pendingExecutionEvents)
	s.Require().NoError(err)

	// Extract the expected transaction IDs in order: process, execute callbacks, system chunk
	expectedSystemTxIDs := make([]flow.Identifier, len(expectedSystemCollection.Transactions))
	for i, tx := range expectedSystemCollection.Transactions {
		expectedSystemTxIDs[i] = tx.ID()
	}

	s.Require().Equal(4, len(expectedSystemTxIDs), "should have 4 system transactions: process + 2 execute callbacks + system chunk")

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
	s.executionAPIClient.
		On("GetTransactionResultsByBlockID", mock.Anything, exeGetTxReq).
		Return(exeGetTxResp, nil).
		Once()

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	executionResult := s.receiptsList[0].ExecutionResult

	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: executionResult.ID(),
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)

	// used to derive transaction status
	s.state.On("Sealed").Return(s.snapshot, nil)
	s.snapshot.On("Head").Return(block.ToHeader(), nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewENTransactionProvider(
		params.Log,
		params.State,
		params.Collections,
		params.ConnFactory,
		params.NodeCommunicator,
		params.TxStatusDeriver,
		params.SystemTxID,
		params.ChainID,
		params.ScheduledCallbacksEnabled,
	)
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	response, metadata, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockId, entities.EventEncodingVersion_CCF_V0, optimistic_sync.Criteria{})

	_ = metadata // TODO: validate metadata

	s.Require().NoError(err)

	// Expected total: user transactions + system transactions (process + 2 execute callbacks + system chunk)
	expectedTotal := len(lightTxResults) + len(expectedSystemTxIDs)
	s.Assert().Equal(expectedTotal, len(response), "should have user txs + system txs")

	// Verify user transactions
	userTxCount := len(lightCol.Transactions)
	for i := 0; i < userTxCount; i++ {
		s.Assert().Equal(lightTxResults[i].TransactionID, response[i].TransactionID)
		s.Assert().Equal(block.Payload.Guarantees[0].CollectionID, response[i].CollectionID)
		s.Assert().Equal(block.ID(), response[i].BlockID)
		s.Assert().Equal(block.Height, response[i].BlockHeight)
		s.Assert().Equal(flow.TransactionStatusSealed, response[i].Status)
	}

	// Verify system collection transactions (all should have ZeroID as collectionID)
	systemTxCount := len(response) - userTxCount
	s.Assert().Equal(len(expectedSystemTxIDs), systemTxCount, "should have 4 system transactions: process + 2 execute callbacks + system chunk")

	for i := 0; i < systemTxCount; i++ {
		systemTxIndex := userTxCount + i
		s.Assert().Equal(flow.ZeroID, response[systemTxIndex].CollectionID)
		s.Assert().Equal(block.ID(), response[systemTxIndex].BlockID)
		s.Assert().Equal(block.Height, response[systemTxIndex].BlockHeight)
		s.Assert().Equal(flow.TransactionStatusSealed, response[systemTxIndex].Status)
		s.Assert().Equal(expectedSystemTxIDs[i], response[userTxCount+i].TransactionID, "system transaction %d should match reconstructed ID", i)
	}
}

// TestTransactionRetry tests that the retry mechanism will send retries at specific times
func (s *Suite) TestTransactionRetry() {
	block := unittest.BlockFixture(
		// Height needs to be at least DefaultTransactionExpiry before we start doing retries
		unittest.Block.WithHeight(flow.DefaultTransactionExpiry + 1),
	)
	transactionBody := unittest.TransactionBodyFixture(unittest.WithReferenceBlock(block.ID()))

	snapshotAtBlock := protocolmock.NewSnapshot(s.T())
	s.state.On("Final").Return(snapshotAtBlock)
	s.state.On("AtBlockID", block.ID()).Return(snapshotAtBlock)
	snapshotAtBlock.On("Head").Return(block.ToHeader(), nil)

	// collection storage returns a not found error
	s.collections.
		On("LightByTransactionID", transactionBody.ID()).
		Return(nil, storage.ErrNotFound)

	client := accessmock.NewAccessAPIClient(s.T())
	params := s.defaultTransactionsParams()
	params.StaticCollectionRPCClient = client
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	retry := retrier.NewRetrier(
		s.log,
		s.blocks,
		s.collections,
		txBackend,
		txBackend.txStatusDeriver,
	)
	retry.RegisterTransaction(block.Height, &transactionBody)

	// Don't retry on every height
	err = retry.Retry(block.Height + 1)
	s.Require().NoError(err)

	client.On("SendTransaction", mock.Anything, mock.Anything).Return(&access.SendTransactionResponse{}, nil).Once()

	// Retry every `retryFrequency`
	err = retry.Retry(block.Height + retrier.RetryFrequency)
	s.Require().NoError(err)

	// do not retry if expired
	err = retry.Retry(block.Height + retrier.RetryFrequency + flow.DefaultTransactionExpiry)
	s.Require().NoError(err)
}

// TestSuccessfulTransactionsDontRetry tests that the retry mechanism will send retries at specific times
func (s *Suite) TestSuccessfulTransactionsDontRetry() {
	collection := unittest.CollectionFixture(1)
	light := collection.Light()
	transactionBody := collection.Transactions[0]
	txID := transactionBody.ID()

	block := unittest.BlockFixture()
	blockID := block.ID()

	s.transactions.On("ByID", txID).Return(transactionBody, nil)
	s.collections.On("LightByTransactionID", txID).Return(light, nil)
	s.blocks.On("ByCollectionID", collection.ID()).Return(block, nil)

	exeEventReq := execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}
	exeEventResp := execproto.GetTransactionResultResponse{
		Events: nil,
	}
	s.executionAPIClient.
		On("GetTransactionResult", context.Background(), &exeEventReq).
		Return(&exeEventResp, status.Errorf(codes.NotFound, "not found")).
		Times(len(s.identities)) // should call each EN once

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
		Times(len(s.identities))

	params := s.defaultTransactionsParams()
	client := accessmock.NewAccessAPIClient(s.T())
	params.StaticCollectionRPCClient = client
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	retry := retrier.NewRetrier(
		s.log,
		s.blocks,
		s.collections,
		txBackend,
		txBackend.txStatusDeriver,
	)
	retry.RegisterTransaction(block.Height, transactionBody)

	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("Collections").Return(s.collections)

	s.execResultProvider.
		On("ExecutionResultInfo", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResultID: unittest.IdentifierFixture(),
			ExecutionNodes:    s.identities.ToSkeleton(),
		}, nil)
	s.executionStateCache.
		On("Snapshot", mock.Anything).
		Return(snapshot, nil)

	// first call - when block under test is greater height than the sealed head, but execution node does not know about Tx
	result, _, err := txBackend.GetTransactionResult(
		context.Background(),
		txID,
		flow.ZeroID,
		flow.ZeroID,
		entities.EventEncodingVersion_JSON_CDC_V0,
		optimistic_sync.Criteria{},
	)
	s.Require().NoError(err)
	s.Require().NotNil(result)

	// status should be finalized since the sealed Blocks is smaller in height
	s.Assert().Equal(flow.TransactionStatusFinalized, result.Status)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + 1)
	s.Require().NoError(err)

	client.AssertNotCalled(s.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + retrier.RetryFrequency)
	s.Require().NoError(err)

	client.AssertNotCalled(s.T(), "SendTransaction", mock.Anything, mock.Anything)

	// Don't retry when block is finalized
	err = retry.Retry(block.Height + retrier.RetryFrequency + flow.DefaultTransactionExpiry)
	s.Require().NoError(err)

	// Should've still should not be called
	client.AssertNotCalled(s.T(), "SendTransaction", mock.Anything, mock.Anything)
}

func (s *Suite) assertTransactionResultResponse(
	err error,
	response *accessmodel.TransactionResult,
	block flow.Block,
	txId flow.Identifier,
	txFailed bool,
	eventsForTx []flow.Event,
) {
	s.Require().NoError(err)
	s.Assert().Equal(block.ID(), response.BlockID)
	s.Assert().Equal(block.Height, response.BlockHeight)
	s.Assert().Equal(txId, response.TransactionID)
	if txId == s.systemTx.ID() {
		s.Assert().Equal(flow.ZeroID, response.CollectionID)
	} else {
		s.Assert().Equal(block.Payload.Guarantees[0].CollectionID, response.CollectionID)
	}
	s.Assert().Equal(len(eventsForTx), len(response.Events))
	// When there are error messages occurred in the transaction, the status should be 1
	if txFailed {
		s.Assert().Equal(uint(1), response.StatusCode)
		s.Assert().Equal(expectedErrorMsg, response.ErrorMessage)
	} else {
		s.Assert().Equal(uint(0), response.StatusCode)
		s.Assert().Equal("", response.ErrorMessage)
	}
	s.Assert().Equal(flow.TransactionStatusSealed, response.Status)
}

// createPendingExecutionEvents creates properly formatted PendingExecution events
// that blueprints.SystemCollection expects for reconstructing the system collection.
func (s *Suite) createPendingExecutionEvents(numCallbacks int) []flow.Event {
	events := make([]flow.Event, numCallbacks)

	// Get system contracts for the test chain
	env := systemcontracts.SystemContractsForChain(s.chainID).AsTemplateEnv()

	for i := 0; i < numCallbacks; i++ {
		// Create the PendingExecution event as it would be emitted by the process callback transaction
		const processedEventTypeTemplate = "A.%v.FlowTransactionScheduler.PendingExecution"
		eventTypeString := fmt.Sprintf(processedEventTypeTemplate, env.FlowTransactionSchedulerAddress)

		// Create Cadence event type
		loc, err := cadenceCommon.HexToAddress(env.FlowTransactionSchedulerAddress)
		s.Require().NoError(err)
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
		s.Require().NoError(err)

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
		s.Require().NoError(err)

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

func (s *Suite) setupExecutionGetEventsRequest(blockID flow.Identifier, blockHeight uint64, events []flow.Event) {
	eventMessages := make([]*entities.Event, len(events))
	for i, event := range events {
		eventMessages[i] = convert.EventToMessage(event)
	}

	request := &execproto.GetEventsForBlockIDsRequest{
		Type:     string(s.processScheduledCallbackEventType),
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

	s.executionAPIClient.
		On("GetEventsForBlockIDs", mock.Anything, request).
		Return(expectedResponse, nil).
		Once()

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()
}
