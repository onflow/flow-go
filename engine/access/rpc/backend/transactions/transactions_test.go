package transactions

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
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
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/retrier"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/execution_result_provider"
	optimisticsyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/badgerimpl"
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

	db                  *badger.DB
	dbDir               string
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter

	executionAPIClient        *accessmock.ExecutionAPIClient
	historicalAccessAPIClient *accessmock.AccessAPIClient

	connectionFactory *connectionmock.ConnectionFactory

	chainID  flow.ChainID
	systemTx *flow.TransactionBody

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList

	execResultProvider  *optimisticsyncmock.ExecutionResultProvider
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
	params := protocolmock.NewParams(s.T())
	params.On("FinalizedRoot").Return(s.rootBlockHeader, nil).Maybe()
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil).Maybe()
	params.On("SporkRootBlockHeight").Return(s.rootBlockHeader.Height, nil).Maybe()
	params.On("SealedRoot").Return(s.rootBlockHeader, nil).Maybe()

	s.state = protocolmock.NewState(s.T())
	s.state.On("Params").Return(params).Maybe()

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

	s.db, s.dbDir = unittest.TempBadgerDB(s.T())
	progress, err := store.NewConsumerProgress(badgerimpl.ToDB(s.db), module.ConsumeProgressLastFullBlockHeight).Initialize(0)
	require.NoError(s.T(), err)
	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(progress)
	s.Require().NoError(err)

	s.execResultProvider = optimisticsyncmock.NewExecutionResultProvider(s.T())
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

func (s *Suite) defaultTransactionsParamsWithExecutionResultProviderMocks() Params {
	executionResult := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(s.rootBlockHeader.ID()))
	s.snapshot.On("SealedResult").Return(executionResult, nil, nil).Once()
	s.state.On("AtBlockID", mock.AnythingOfType("flow.Identifier")).Return(s.snapshot).Once()
	s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(s.rootBlockHeader.ID(), nil).Once()

	return s.defaultTransactionsParams()
}

func (s *Suite) defaultTransactionsParams() Params {
	nodeProvider := rpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.state,
		s.receipts,
		s.preferredExecutionNodeIDs,
		s.fixedExecutionNodeIDs,
	)

	executionResultProvider, err := execution_result_provider.NewExecutionResultProvider(
		s.log,
		s.state,
		s.headers,
		s.receipts,
		execution_result_provider.NewExecutionNodes(s.preferredExecutionNodeIDs, s.fixedExecutionNodeIDs),
		s.operatorCriteria,
	)
	s.Require().NoError(err)

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
		executionResultProvider,
		txStatusDeriver,
		s.systemTx.ID(),
		s.systemTx,
	)

	return Params{
		Log:                         s.log,
		Metrics:                     metrics.NewNoopCollector(),
		State:                       s.state,
		ChainID:                     flow.Testnet,
		SystemTxID:                  s.systemTx.ID(),
		SystemTx:                    s.systemTx,
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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
func (s *Suite) TestGetSystemTransaction_HappyPath() {
	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
	txBackend, err := NewTransactionsBackend(params)
	require.NoError(s.T(), err)

	block := unittest.BlockFixture()
	res, err := txBackend.GetSystemTransaction(context.Background(), block.ID())
	s.Require().NoError(err)

	systemTx, err := blueprints.SystemChunkTransaction(s.chainID.Chain())
	s.Require().NoError(err)

	s.Require().Equal(systemTx, res)
}

func (s *Suite) TestGetSystemTransactionResult_HappyPath() {
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

		s.state.
			On("AtBlockID", blockID).
			Return(stateSnapshotForKnownBlock, nil).
			Once()

		// block storage returns the corresponding block
		s.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		s.headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(blockID, nil)

		stateSnapshotForKnownBlock.On("SealedResult").Return(&s.receiptsList[0].ExecutionResult, nil, nil)

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

		s.executionAPIClient.
			On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *execproto.GetTransactionResultRequest) bool {
				txID := s.systemTx.ID()
				return bytes.Equal(txID[:], req.TransactionId)
			})).
			Return(exeEventResp.TransactionResults[0], nil).
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
			On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
			Return(&optimistic_sync.ExecutionResultInfo{
				ExecutionResult: &s.receiptsList[0].ExecutionResult,
				ExecutionNodes:  s.identities.ToSkeleton(),
			}, nil)

		res, _, err := backend.GetSystemTransactionResult(
			context.Background(),
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
		func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
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

func (s *Suite) TestGetSystemTransactionResultFromStorage() {
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
	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResult", blockId, reqCriteria).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResult: fakeRes}, nil)
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
		s.execResultProvider,
		s.executionStateCache,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)
	response, _, err := txBackend.GetSystemTransactionResult(
		context.Background(),
		blockId,
		entities.EventEncodingVersion_JSON_CDC_V0,
		reqCriteria,
	)
	s.assertTransactionResultResponse(err, response, *block, txId, lightResult.Failed, eventsForTx)
}

// TestGetSystemTransactionResult_BlockNotFound tests GetSystemTransactionResult function when block was not found.
func (s *Suite) TestGetSystemTransactionResult_BlockNotFound() {
	block := unittest.BlockFixture()
	s.blocks.
		On("ByID", block.ID()).
		Return(nil, storage.ErrNotFound).
		Once()

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)
	res, _, err := txBackend.GetSystemTransactionResult(
		context.Background(),
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

	exeEventResp := &execproto.GetTransactionResultsResponse{
		TransactionResults: []*execproto.GetTransactionResultResponse{{
			Events:               eventMessages,
			EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
		}},
	}

	s.executionAPIClient.
		On("GetTransactionResult", mock.Anything, mock.MatchedBy(func(req *execproto.GetTransactionResultRequest) bool {
			txID := s.systemTx.ID()
			return bytes.Equal(txID[:], req.TransactionId)
		})).
		Return(exeEventResp.TransactionResults[0], nil).
		Once()

	s.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(s.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	s.execResultProvider.
		On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResult: &s.receiptsList[0].ExecutionResult,
			ExecutionNodes:  s.identities.ToSkeleton(),
		}, nil)

	res, _, err := txBackend.GetSystemTransactionResult(
		context.Background(),
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()

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
		On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResult: fakeRes,
			ExecutionNodes:  s.identities.ToSkeleton(),
		}, nil)
	s.executionStateCache.On("Snapshot", mock.Anything).Return(snapshot, nil)

	params.TxProvider = provider.NewLocalTransactionProvider(
		params.State,
		params.Collections,
		params.Blocks,
		params.SystemTxID,
		params.TxStatusDeriver,
		s.execResultProvider,
		s.executionStateCache,
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResult: fakeRes}, nil)
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
		s.execResultProvider,
		s.executionStateCache,
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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()

	// Set up optimistic sync mocks to provide snapshot-backed readers
	snapshot := optimisticsyncmock.NewSnapshot(s.T())
	snapshot.On("BlockStatus").Return(flow.BlockStatusSealed)

	// ExecutionResultQuery can return any result with an ID; only ID is used to fetch snapshot
	fakeRes := &flow.ExecutionResult{PreviousResultID: unittest.IdentifierFixture()}
	s.execResultProvider.
		On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{ExecutionResult: fakeRes}, nil)
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
		s.execResultProvider,
		s.executionStateCache,
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

// TestTransactionRetry tests that the retry mechanism will send retries at specific times
func (s *Suite) TestTransactionRetry() {
	block := unittest.BlockFixture(
		// Height needs to be at least DefaultTransactionExpiry before we start doing retries
		unittest.Block.WithHeight(flow.DefaultTransactionExpiry + 1),
	)
	transactionBody := unittest.TransactionBodyFixture(unittest.WithReferenceBlock(block.ID()))

	// execution result provider bootstrap
	s.headers.On("BlockIDByHeight", s.rootBlockHeader.Height).Return(s.rootBlockHeader.ID(), nil).Once()
	snapshotAtRoot := protocolmock.NewSnapshot(s.T())
	snapshotAtRoot.On("SealedResult").Return(unittest.ExecutionResultFixture(), nil, nil)
	s.state.On("AtBlockID", s.rootBlockHeader.ID()).Return(snapshotAtRoot)

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

	params := s.defaultTransactionsParamsWithExecutionResultProviderMocks()
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
		On("ExecutionResult", block.ID(), mock.AnythingOfType("optimistic_sync.Criteria")).
		Return(&optimistic_sync.ExecutionResultInfo{
			ExecutionResult: unittest.ExecutionResultFixture(),
			ExecutionNodes:  s.identities.ToSkeleton(),
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

func (s *Suite) setupReceipts(block *flow.Block) ([]*flow.ExecutionReceipt, flow.IdentityList) {
	receipt1 := unittest.ReceiptForBlockFixture(block)
	receipt1.ExecutorID = s.identities[0].NodeID
	receipt2 := unittest.ReceiptForBlockFixture(block)
	receipt2.ExecutorID = s.identities[1].NodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult

	receipts := flow.ExecutionReceiptList{receipt1, receipt2}
	s.receipts.
		On("ByBlockID", block.ID()).
		Return(receipts, nil)

	return receipts, s.identities
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
