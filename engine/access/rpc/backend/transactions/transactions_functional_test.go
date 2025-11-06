package transactions

import (
	"context"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/onflow/flow/protobuf/go/flow/execution"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/provider"
	txstatus "github.com/onflow/flow-go/engine/access/rpc/backend/transactions/status"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/system"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	testutil "github.com/onflow/flow-go/module/executiondatasync/testutil"
	"github.com/onflow/flow-go/module/metrics"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/inmem"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestTransactionsFunctionalSuite(t *testing.T) {
	suite.Run(t, new(TransactionsFunctionalSuite))
}

// TransactionsFunctionalSuite tests implements functional happy path tests of the transaction backend
// for local and execution node providers. The tests use a full database with real storages.
//
// Mocking is only used for the following components:
//   - Execution node backends
//   - Index reporter - to avoid needing to run a registers db
//   - State for the execution node provider to avoid issues having to needing to deal with identities
//     when selecting execution nodes for each request.
//
// Not all methods are tested, just methods required to exercise each of the provider methods. Detailed
// per-method testing is implemented in transactions_test.go.
type TransactionsFunctionalSuite struct {
	suite.Suite

	log         zerolog.Logger
	g           *fixtures.GeneratorSuite
	db          storage.DB
	lockManager lockctx.Manager

	blocks                storage.Blocks
	collections           storage.Collections
	transactions          storage.Transactions
	events                storage.Events
	results               storage.LightTransactionResults
	receipts              storage.ExecutionReceipts
	txErrorMessages       storage.TransactionResultErrorMessages
	scheduledTransactions storage.ScheduledTransactions

	eventsIndex            *index.EventsIndex
	txResultsIndex         *index.TransactionResultsIndex
	validatorBlocks        *validator.ProtocolStateBlocks
	txErrorMessageProvider error_messages.Provider

	state               *protocol.State
	rootSnapshot        *inmem.Snapshot
	participants        flow.IdentityList
	lastFullBlockHeight *counters.PersistentStrictMonotonicCounter
	txStatusDeriver     *txstatus.TxStatusDeriver
	nodeProvider        *commonrpc.ExecutionNodeIdentitiesProvider
	reporter            *syncmock.IndexReporter

	rootBlock        *flow.Block
	tf               *testutil.TestFixture
	systemCollection *system.SystemCollection

	mockState  *protocolmock.State
	execClient *accessmock.ExecutionAPIClient
}

func (s *TransactionsFunctionalSuite) SetupTest() {
	s.g = fixtures.NewGeneratorSuite()

	s.log = unittest.Logger()
	metrics := metrics.NewNoopCollector()

	// Setup database
	s.lockManager = storage.NewTestingLockManager()

	dbDir := unittest.TempDir(s.T())
	s.T().Cleanup(func() { s.Require().NoError(os.RemoveAll(dbDir)) })

	pdb := unittest.PebbleDB(s.T(), dbDir)
	s.T().Cleanup(func() { s.Require().NoError(pdb.Close()) })

	s.db = pebbleimpl.ToDB(pdb)

	// Instantiate storages
	all := store.InitAll(metrics, s.db)

	s.blocks = all.Blocks
	s.collections = all.Collections
	s.transactions = all.Transactions
	s.receipts = all.Receipts
	s.events = store.NewEvents(metrics, s.db)
	s.results = store.NewLightTransactionResults(metrics, s.db, bstorage.DefaultCacheSize)
	s.txErrorMessages = store.NewTransactionResultErrorMessages(metrics, s.db, bstorage.DefaultCacheSize)
	s.scheduledTransactions = store.NewScheduledTransactions(metrics, s.db, bstorage.DefaultCacheSize)

	s.reporter = syncmock.NewIndexReporter(s.T())

	reporter := index.NewReporter()
	err := reporter.Initialize(s.reporter)
	s.Require().NoError(err)

	s.eventsIndex = index.NewEventsIndex(reporter, s.events)
	s.txResultsIndex = index.NewTransactionResultsIndex(reporter, s.results)
	s.validatorBlocks = validator.NewProtocolStateBlocks(s.state, reporter)

	s.txErrorMessageProvider = error_messages.NewTxErrorMessageProvider(s.log, s.txErrorMessages, nil, nil, nil, nil)

	s.participants = s.g.Identities().List(5, fixtures.Identity.WithAllRoles())
	s.rootSnapshot = unittest.RootSnapshotFixtureWithChainID(s.participants, s.g.ChainID())

	s.state, err = protocol.Bootstrap(
		metrics,
		s.db,
		s.lockManager,
		all.Headers,
		all.Seals,
		all.Results,
		all.Blocks,
		all.QuorumCertificates,
		all.EpochSetups,
		all.EpochCommits,
		all.EpochProtocolStateEntries,
		all.ProtocolKVStore,
		all.VersionBeacons,
		s.rootSnapshot,
	)
	s.Require().NoError(err)

	// Generate fixture data
	s.systemCollection, err = system.DefaultSystemCollection(s.g.ChainID(), true)
	s.Require().NoError(err)

	s.rootBlock = s.state.Params().SporkRootBlock()

	s.tf = testutil.CompleteFixture(s.T(), s.g, s.rootBlock)

	block := s.tf.Block
	blockID := s.tf.Block.ID()

	// Populate the database
	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			if err := all.EpochProtocolStateEntries.BatchIndex(lctx, rw, blockID, block.Payload.ProtocolStateID); err != nil {
				return err
			}

			return s.blocks.BatchStore(lctx, rw, unittest.ProposalFromBlock(block))
		})
	})
	s.Require().NoError(err)

	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertCollection, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, collection := range s.tf.ExpectedCollections {
				if _, err := s.collections.BatchStoreAndIndexByTransaction(lctx, collection, rw); err != nil {
					return err
				}
			}
			return nil
		})
	})
	s.Require().NoError(err)

	err = unittest.WithLock(s.T(), s.lockManager, storage.LockIndexBlockByPayloadGuarantees, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.blocks.BatchIndexBlockContainingCollectionGuarantees(lctx, rw, blockID, flow.GetIDs(block.Payload.Guarantees))
		})
	})
	s.Require().NoError(err)

	err = unittest.WithLocks(s.T(), s.lockManager, []string{
		storage.LockInsertLightTransactionResult,
		storage.LockIndexScheduledTransaction,
	}, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			if err := s.results.BatchStore(lctx, rw, blockID, s.tf.ExpectedResults); err != nil {
				return err
			}

			for txID, scheduledTxID := range s.tf.ExpectedScheduledTransactions {
				if err := s.scheduledTransactions.BatchIndex(lctx, blockID, txID, scheduledTxID, rw); err != nil {
					return err
				}
			}

			return nil
		})
	})
	s.Require().NoError(err)

	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertEvent, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.events.BatchStore(lctx, blockID, []flow.EventsList{s.tf.ExpectedEvents}, rw)
		})
	})
	s.Require().NoError(err)

	err = unittest.WithLock(s.T(), s.lockManager, storage.LockInsertTransactionResultErrMessage, func(lctx lockctx.Context) error {
		return s.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return s.txErrorMessages.BatchStore(lctx, rw, blockID, s.tf.TxErrorMessages)
		})
	})
	s.Require().NoError(err)

	lastFullBlockHeightProgress, err := store.NewConsumerProgress(s.db, module.ConsumeProgressLastFullBlockHeight).Initialize(s.rootBlock.Height)
	s.Require().NoError(err)

	s.lastFullBlockHeight, err = counters.NewPersistentStrictMonotonicCounter(lastFullBlockHeightProgress)
	s.Require().NoError(err)

	// Instantiate intermediate components
	s.txStatusDeriver = txstatus.NewTxStatusDeriver(s.state, s.lastFullBlockHeight)

	s.mockState = protocolmock.NewState(s.T())
	s.nodeProvider = commonrpc.NewExecutionNodeIdentitiesProvider(s.log, s.mockState, s.receipts, nil, nil)

	s.execClient = accessmock.NewExecutionAPIClient(s.T())
}

func (s *TransactionsFunctionalSuite) defaultTransactionsParams() Params {
	txValidator, err := validator.NewTransactionValidator(
		s.validatorBlocks,
		s.g.ChainID().Chain(),
		metrics.NewNoopCollector(),
		validator.TransactionValidationOptions{},
		execmock.NewScriptExecutor(s.T()),
	)
	s.Require().NoError(err)

	return Params{
		Log:                          s.log,
		Metrics:                      metrics.NewNoopCollector(),
		ChainID:                      s.g.ChainID(),
		State:                        s.state,
		SystemCollection:             s.systemCollection,
		NodeProvider:                 s.nodeProvider,
		Blocks:                       s.blocks,
		Collections:                  s.collections,
		Transactions:                 s.transactions,
		ScheduledTransactions:        s.scheduledTransactions,
		TxErrorMessageProvider:       s.txErrorMessageProvider,
		TxValidator:                  txValidator,
		TxStatusDeriver:              s.txStatusDeriver,
		EventsIndex:                  s.eventsIndex,
		TxResultsIndex:               s.txResultsIndex,
		ScheduledTransactionsEnabled: true,
	}
}

func (s *TransactionsFunctionalSuite) defaultExecutionNodeParams() Params {
	blockID := s.tf.Block.ID()

	connectionFactory := connectionmock.NewConnectionFactory(s.T())
	connectionFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mocks.MockCloser{}, nil)
	nodeCommunicator := node_communicator.NewNodeCommunicator(false)

	stateParams := protocolmock.NewParams(s.T())
	stateParams.On("FinalizedRoot").Return(s.rootBlock.ToHeader())
	s.mockState.On("Params").Return(stateParams)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewENTransactionProvider(
		s.log,
		s.state,
		s.collections,
		connectionFactory,
		nodeCommunicator,
		s.nodeProvider,
		s.txStatusDeriver,
		s.systemCollection,
		s.g.ChainID(),
		true,
	)

	snapshot := protocolmock.NewSnapshot(s.T())
	snapshot.On("Identities", mock.Anything).Return(func(filter flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
		return s.participants.Filter(filter), nil
	})
	s.mockState.On("AtBlockID", blockID).Return(snapshot)

	finalizedSnapshot := protocolmock.NewSnapshot(s.T())
	finalizedSnapshot.On("Identities", mock.Anything).Return(func(filter flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
		return s.participants.Filter(filter), nil
	})
	s.mockState.On("Final").Return(finalizedSnapshot)

	return params
}

func eventsForTransaction(events flow.EventsList, txID flow.Identifier) flow.EventsList {
	filtered := make(flow.EventsList, 0)
	for _, event := range events {
		if event.TransactionID == txID {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func (s *TransactionsFunctionalSuite) expectedResultForIndex(index int, encodingVersion entities.EventEncodingVersion) *accessmodel.TransactionResult {
	block := s.tf.Block
	blockID := s.tf.Block.ID()

	txResult := s.tf.ExpectedResults[index]
	txID := txResult.TransactionID

	txCount := 0
	collectionID := flow.ZeroID
	for _, collection := range s.tf.ExpectedCollections {
		if index < txCount+len(collection.Transactions) {
			collectionID = collection.ID()
			break
		}
		txCount += len(collection.Transactions)
	}
	// if the tx is a system tx, its index is greater than the total number of transactions, so the
	// collection ID will default to flow.ZeroID.

	events := eventsForTransaction(s.tf.ExpectedEvents, txID)
	if encodingVersion == entities.EventEncodingVersion_JSON_CDC_V0 {
		convertedEvents, err := convert.CcfEventsToJsonEvents(events)
		s.Require().NoError(err)

		events = convertedEvents
	}

	errorMessage := ""
	statusCode := uint(0)
	if txResult.Failed {
		statusCode = uint(1)
		for _, txErrorMessage := range s.tf.TxErrorMessages {
			if txErrorMessage.TransactionID == txID {
				errorMessage = txErrorMessage.ErrorMessage
				break
			}
		}
		s.Require().NotEmpty(errorMessage)
	}

	return &accessmodel.TransactionResult{
		TransactionID: txID,
		Status:        flow.TransactionStatusExecuted,
		StatusCode:    statusCode,
		Events:        events,
		ErrorMessage:  errorMessage,
		BlockID:       blockID,
		BlockHeight:   block.Height,
		CollectionID:  collectionID,
	}
}

func (s *TransactionsFunctionalSuite) TestTransactionResult_Local() {
	block := s.tf.Block
	blockID := s.tf.Block.ID()

	collection := s.tf.ExpectedCollections[0]
	collectionID := collection.ID()

	txID := s.tf.ExpectedResults[1].TransactionID

	expectedResult := s.expectedResultForIndex(1, entities.EventEncodingVersion_JSON_CDC_V0)
	s.reporter.On("HighestIndexedHeight").Return(block.Height, nil)
	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventsIndex,
		s.txResultsIndex,
		s.txErrorMessageProvider,
		s.systemCollection,
		s.txStatusDeriver,
		s.g.ChainID(),
		true,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	result, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, collectionID, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
}

func (s *TransactionsFunctionalSuite) TestTransactionResultByIndex_Local() {
	block := s.tf.Block
	blockID := s.tf.Block.ID()

	expectedResult := s.expectedResultForIndex(1, entities.EventEncodingVersion_JSON_CDC_V0)
	s.reporter.On("HighestIndexedHeight").Return(block.Height, nil)
	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventsIndex,
		s.txResultsIndex,
		s.txErrorMessageProvider,
		s.systemCollection,
		s.txStatusDeriver,
		s.g.ChainID(),
		true,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	result, err := txBackend.GetTransactionResultByIndex(context.Background(), blockID, 1, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
}

func (s *TransactionsFunctionalSuite) TestTransactionResultsByBlockID_Local() {
	block := s.tf.Block
	blockID := s.tf.Block.ID()

	expectedResults := make([]*accessmodel.TransactionResult, len(s.tf.ExpectedResults))
	for i := range s.tf.ExpectedResults {
		expectedResults[i] = s.expectedResultForIndex(i, entities.EventEncodingVersion_JSON_CDC_V0)
	}

	s.reporter.On("HighestIndexedHeight").Return(block.Height, nil)
	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventsIndex,
		s.txResultsIndex,
		s.txErrorMessageProvider,
		s.systemCollection,
		s.txStatusDeriver,
		s.g.ChainID(),
		true,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	results, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockID, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResults, results)
}

func (s *TransactionsFunctionalSuite) TestTransactionsByBlockID_Local() {
	block := s.tf.Block
	blockID := block.ID()

	expectedTransactions := make([]*flow.TransactionBody, 0, len(s.tf.ExpectedResults))
	for _, collection := range s.tf.ExpectedCollections {
		expectedTransactions = append(expectedTransactions, collection.Transactions...)
	}

	systemCollection, err := blueprints.SystemCollection(s.g.ChainID().Chain(), s.tf.ExpectedEvents)
	s.Require().NoError(err)
	expectedTransactions = append(expectedTransactions, systemCollection.Transactions...)

	s.reporter.On("HighestIndexedHeight").Return(block.Height, nil)
	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventsIndex,
		s.txResultsIndex,
		s.txErrorMessageProvider,
		s.systemCollection,
		s.txStatusDeriver,
		s.g.ChainID(),
		true,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	results, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
	s.Require().NoError(err)
	s.Require().Equal(expectedTransactions, results)
}

func (s *TransactionsFunctionalSuite) TestScheduledTransactionsByBlockID_Local() {
	block := s.tf.Block

	systemCollection, err := system.NewSystemCollection(s.g.ChainID(), s.tf.ExpectedEvents)
	s.Require().NoError(err)

	s.reporter.On("HighestIndexedHeight").Return(block.Height, nil)
	s.reporter.On("LowestIndexedHeight").Return(s.rootBlock.Height, nil)

	params := s.defaultTransactionsParams()
	params.TxProvider = provider.NewLocalTransactionProvider(
		s.state,
		s.collections,
		s.blocks,
		s.eventsIndex,
		s.txResultsIndex,
		s.txErrorMessageProvider,
		s.systemCollection,
		s.txStatusDeriver,
		s.g.ChainID(),
		true,
	)

	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	for txID, scheduledTxID := range s.tf.ExpectedScheduledTransactions {
		expectedTransaction, ok := systemCollection.ByID(txID)
		s.Require().True(ok)

		results, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		s.Require().NoError(err)
		s.Require().Equal(expectedTransaction, results)

		break // call for the first scheduled transaction iterated
	}
}

func (s *TransactionsFunctionalSuite) TestTransactionResult_ExecutionNode() {
	blockID := s.tf.Block.ID()

	collection := s.tf.ExpectedCollections[0]
	collectionID := collection.ID()

	txID := s.tf.ExpectedResults[1].TransactionID

	accessResponse := convert.TransactionResultToMessage(s.expectedResultForIndex(1, entities.EventEncodingVersion_CCF_V0))
	nodeResponse := &execution.GetTransactionResultResponse{
		StatusCode:           accessResponse.StatusCode,
		ErrorMessage:         accessResponse.ErrorMessage,
		Events:               accessResponse.Events,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}
	expectedResult := s.expectedResultForIndex(1, entities.EventEncodingVersion_JSON_CDC_V0)

	expectedRequest := &execproto.GetTransactionResultRequest{
		BlockId:       blockID[:],
		TransactionId: txID[:],
	}

	s.execClient.
		On("GetTransactionResult", mock.Anything, expectedRequest).
		Return(nodeResponse, nil)

	params := s.defaultExecutionNodeParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	result, err := txBackend.GetTransactionResult(context.Background(), txID, blockID, collectionID, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
}

func (s *TransactionsFunctionalSuite) TestTransactionResultByIndex_ExecutionNode() {
	blockID := s.tf.Block.ID()

	accessResponse := convert.TransactionResultToMessage(s.expectedResultForIndex(1, entities.EventEncodingVersion_CCF_V0))
	nodeResponse := &execution.GetTransactionResultResponse{
		StatusCode:           accessResponse.StatusCode,
		ErrorMessage:         accessResponse.ErrorMessage,
		Events:               accessResponse.Events,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}
	expectedResult := s.expectedResultForIndex(1, entities.EventEncodingVersion_JSON_CDC_V0)

	expectedRequest := &execproto.GetTransactionByIndexRequest{
		BlockId: blockID[:],
		Index:   1,
	}

	s.execClient.
		On("GetTransactionResultByIndex", mock.Anything, expectedRequest).
		Return(nodeResponse, nil)

	params := s.defaultExecutionNodeParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	result, err := txBackend.GetTransactionResultByIndex(context.Background(), blockID, 1, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResult, result)
}

func (s *TransactionsFunctionalSuite) TestTransactionResultsByBlockID_ExecutionNode() {
	blockID := s.tf.Block.ID()

	expectedResults := make([]*accessmodel.TransactionResult, len(s.tf.ExpectedResults))
	nodeResults := make([]*execution.GetTransactionResultResponse, len(s.tf.ExpectedResults))
	for i := range s.tf.ExpectedResults {
		accessResponse := convert.TransactionResultToMessage(s.expectedResultForIndex(i, entities.EventEncodingVersion_CCF_V0))
		nodeResults[i] = &execution.GetTransactionResultResponse{
			StatusCode:   accessResponse.StatusCode,
			ErrorMessage: accessResponse.ErrorMessage,
			Events:       accessResponse.Events,
		}
		expectedResults[i] = s.expectedResultForIndex(i, entities.EventEncodingVersion_JSON_CDC_V0)
	}

	nodeResponse := &execution.GetTransactionResultsResponse{
		TransactionResults:   nodeResults,
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}

	expectedRequest := &execproto.GetTransactionsByBlockIDRequest{
		BlockId: blockID[:],
	}

	s.execClient.
		On("GetTransactionResultsByBlockID", mock.Anything, expectedRequest).
		Return(nodeResponse, nil)

	params := s.defaultExecutionNodeParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	result, err := txBackend.GetTransactionResultsByBlockID(context.Background(), blockID, entities.EventEncodingVersion_JSON_CDC_V0)
	s.Require().NoError(err)
	s.Require().Equal(expectedResults, result)
}

func (s *TransactionsFunctionalSuite) TestTransactionsByBlockID_ExecutionNode() {
	block := s.tf.Block
	blockID := block.ID()

	expectedTransactions := make([]*flow.TransactionBody, 0, len(s.tf.ExpectedResults))
	for _, collection := range s.tf.ExpectedCollections {
		expectedTransactions = append(expectedTransactions, collection.Transactions...)
	}

	systemCollection, err := blueprints.SystemCollection(s.g.ChainID().Chain(), s.tf.ExpectedEvents)
	s.Require().NoError(err)
	expectedTransactions = append(expectedTransactions, systemCollection.Transactions...)

	env := systemcontracts.SystemContractsForChain(s.g.ChainID()).AsTemplateEnv()
	pendingExecuteEventType := blueprints.PendingExecutionEventType(env)

	expectedRequest := &execproto.GetEventsForBlockIDsRequest{
		Type:     string(pendingExecuteEventType),
		BlockIds: [][]byte{blockID[:]},
	}

	events := make([]*entities.Event, 0)
	for _, event := range s.tf.ExpectedEvents {
		if blueprints.IsPendingExecutionEvent(env, event) {
			events = append(events, convert.EventToMessage(event))
		}
	}

	nodeResponse := &execution.GetEventsForBlockIDsResponse{
		Results: []*execution.GetEventsForBlockIDsResponse_Result{{
			BlockId:     blockID[:],
			BlockHeight: block.Height,
			Events:      events,
		}},
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}

	s.execClient.
		On("GetEventsForBlockIDs", mock.Anything, expectedRequest).
		Return(nodeResponse, nil)

	params := s.defaultExecutionNodeParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	results, err := txBackend.GetTransactionsByBlockID(context.Background(), blockID)
	s.Require().NoError(err)
	s.Require().Equal(expectedTransactions, results)
}

func (s *TransactionsFunctionalSuite) TestScheduledTransactionsByBlockID_ExecutionNode() {
	block := s.tf.Block
	blockID := block.ID()

	systemCollection, err := system.NewSystemCollection(s.g.ChainID(), s.tf.ExpectedEvents)
	s.Require().NoError(err)

	env := systemcontracts.SystemContractsForChain(s.g.ChainID()).AsTemplateEnv()
	pendingExecuteEventType := blueprints.PendingExecutionEventType(env)

	expectedRequest := &execproto.GetEventsForBlockIDsRequest{
		Type:     string(pendingExecuteEventType),
		BlockIds: [][]byte{blockID[:]},
	}

	events := make([]*entities.Event, 0)
	for _, event := range s.tf.ExpectedEvents {
		if blueprints.IsPendingExecutionEvent(env, event) {
			events = append(events, convert.EventToMessage(event))
		}
	}

	nodeResponse := &execution.GetEventsForBlockIDsResponse{
		Results: []*execution.GetEventsForBlockIDsResponse_Result{{
			BlockId:     blockID[:],
			BlockHeight: block.Height,
			Events:      events,
		}},
		EventEncodingVersion: entities.EventEncodingVersion_CCF_V0,
	}

	s.execClient.
		On("GetEventsForBlockIDs", mock.Anything, expectedRequest).
		Return(nodeResponse, nil)

	params := s.defaultExecutionNodeParams()
	txBackend, err := NewTransactionsBackend(params)
	s.Require().NoError(err)

	for txID, scheduledTxID := range s.tf.ExpectedScheduledTransactions {
		expectedTransaction, ok := systemCollection.ByID(txID)
		s.Require().True(ok)

		results, err := txBackend.GetScheduledTransaction(context.Background(), scheduledTxID)
		s.Require().NoError(err)
		s.Require().Equal(expectedTransaction, results)

		break // call for the first scheduled transaction iterated
	}
}
