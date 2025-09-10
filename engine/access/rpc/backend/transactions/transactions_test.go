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
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
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

	chainID  flow.ChainID
	systemTx *flow.TransactionBody

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestTransactionsBackend(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.New(zerolog.NewConsoleWriter())
	suite.snapshot = protocolmock.NewSnapshot(suite.T())

	header := unittest.BlockHeaderFixture()
	params := protocolmock.NewParams(suite.T())
	params.On("FinalizedRoot").Return(header, nil).Maybe()
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil).Maybe()
	params.On("SporkRootBlockHeight").Return(header.Height, nil).Maybe()
	params.On("SealedRoot").Return(header, nil).Maybe()

	suite.state = protocolmock.NewState(suite.T())
	suite.state.On("Params").Return(params).Maybe()

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

	suite.systemTx, err = blueprints.SystemChunkTransaction(flow.Testnet.Chain())
	suite.Require().NoError(err)

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
		suite.systemTx.ID(),
		suite.systemTx,
	)

	return Params{
		Log:                         suite.log,
		Metrics:                     metrics.NewNoopCollector(),
		State:                       suite.state,
		ChainID:                     flow.Testnet,
		SystemTxID:                  suite.systemTx.ID(),
		SystemTx:                    suite.systemTx,
		StaticCollectionRPCClient:   suite.historicalAccessAPIClient,
		HistoricalAccessNodeClients: nil,
		NodeCommunicator:            nodeCommunicator,
		ConnFactory:                 suite.connectionFactory,
		EnableRetries:               true,
		NodeProvider:                nodeProvider,
		Blocks:                      suite.blocks,
		Collections:                 suite.collections,
		Transactions:                suite.transactions,
		TxErrorMessageProvider:      suite.errorMessageProvider,
		TxResultCache:               suite.txResultCache,
		TxProvider:                  txProvider,
		TxValidator:                 txValidator,
		TxStatusDeriver:             txStatusDeriver,
		EventsIndex:                 suite.eventsIndex,
		TxResultsIndex:              suite.txResultsIndex,
	}
}

// TestGetTransactionResult_UnknownTx returns unknown result when tx not found
func (suite *Suite) TestGetTransactionResult_UnknownTx() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

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
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

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
	suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to find: %v", expectedErr))
}

// TestGetTransactionResult_HistoricNodes_Success tests lookup in historic nodes
func (suite *Suite) TestGetTransactionResult_HistoricNodes_Success() {
	block := unittest.BlockFixture()
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody
	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})

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
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody

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

	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})
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
	tbody := unittest.TransactionBodyFixture()
	tx := unittest.TransactionFixture()
	tx.TransactionBody = tbody

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

	coll := unittest.CollectionFromTransactions([]*flow.Transaction{&tx})
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
func (suite *Suite) TestGetSystemTransaction_HappyPath() {
	params := suite.defaultTransactionsParams()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(suite.T(), err)

	block := unittest.BlockFixture()
	res, err := txBackend.GetSystemTransaction(context.Background(), block.ID())
	suite.Require().NoError(err)

	systemTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
	suite.Require().NoError(err)

	suite.Require().Equal(systemTx, res)
}

func (suite *Suite) TestGetSystemTransactionResult_HappyPath() {
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

		exeEventResp := &execproto.GetTransactionResultsResponse{
			TransactionResults: []*execproto.GetTransactionResultResponse{{
				Events:               eventMessages,
				EventEncodingVersion: exeNodeEventEncodingVersion,
			}},
			EventEncodingVersion: exeNodeEventEncodingVersion,
		}

		suite.executionAPIClient.
			On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *execproto.GetTransactionResultRequest) bool {
				txID := suite.systemTx.ID()
				return bytes.Equal(txID[:], req.TransactionId)
			})).
			Return(exeEventResp.TransactionResults[0], nil).
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
			block.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)

		// Expected system chunk transaction
		suite.Require().Equal(flow.TransactionStatusExecuted, res.Status)
		suite.Require().Equal(suite.systemTx.ID(), res.TransactionID)

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

func (suite *Suite) TestGetSystemTransactionResultFromStorage() {
	block := unittest.BlockFixture()
	sysTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
	suite.Require().NoError(err)
	suite.Require().NotNil(sysTx)
	txId := suite.systemTx.ID()
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
		params.SystemTxID,
		params.TxStatusDeriver,
	)

	txBackend, err := NewTransactionsBackend(params)
	suite.Require().NoError(err)
	response, err := txBackend.GetSystemTransactionResult(context.Background(), blockId, entities.EventEncodingVersion_JSON_CDC_V0)
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

	exeEventResp := &execproto.GetTransactionResultsResponse{
		TransactionResults: []*execproto.GetTransactionResultResponse{{
			Events:               eventMessages,
			EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
		}},
	}

	suite.executionAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *execproto.GetTransactionResultRequest) bool {
			txID := suite.systemTx.ID()
			return bytes.Equal(txID[:], req.TransactionId)
		})).
		Return(exeEventResp.TransactionResults[0], nil).
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
	transaction := unittest.TransactionFixture()
	col := unittest.CollectionFromTransactions([]*flow.Transaction{&transaction})
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
		Return(&transaction.TransactionBody, nil)

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
		params.SystemTxID,
		params.TxStatusDeriver,
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
		params.SystemTxID,
		params.TxStatusDeriver,
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
		TransactionID:   suite.systemTx.ID(),
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
		params.SystemTxID,
		params.TxStatusDeriver,
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
	if txId == suite.systemTx.ID() {
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
