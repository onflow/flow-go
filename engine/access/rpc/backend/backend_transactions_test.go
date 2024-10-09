package backend

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/dgraph-io/badger/v2"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acc "github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/index"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
)

const expectedErrorMsg = "expected test error"

func (suite *Suite) withPreConfiguredState(f func(snap protocol.Snapshot)) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolStateAndMutator(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState, mutableState protocol.MutableProtocolState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), mutableState, state)

		epochBuilder.
			BuildEpoch().
			CompleteEpoch()

		// get heights of each phase in built epochs
		epoch1, ok := epochBuilder.EpochHeights(1)
		require.True(suite.T(), ok)

		// setup AtHeight mock returns for State
		for _, height := range epoch1.Range() {
			suite.state.On("AtHeight", epoch1.Range()).Return(state.AtHeight(height))
		}

		snap := state.AtHeight(epoch1.FinalHeight())
		suite.state.On("Final").Return(snap)
		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil).Once()

		f(snap)
	})
}

// TestGetTransactionResultReturnsUnknown returns unknown result when tx not found
func (suite *Suite) TestGetTransactionResultReturnsUnknown() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})
		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		params := suite.defaultBackendParams()
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		res, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(res.Status, flow.TransactionStatusUnknown)
	})
}

// TestGetTransactionResultReturnsTransactionError returns error from transaction storage
func (suite *Suite) TestGetTransactionResultReturnsTransactionError() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, fmt.Errorf("some other error"))

		suite.blocks.
			On("ByID", block.ID()).
			Return(&block, nil).
			Once()

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		params := suite.defaultBackendParams()
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		_, err = backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to find: %v", fmt.Errorf("some other error")))
	})
}

// TestGetTransactionResultReturnsValidTransactionResultFromHistoricNode tests lookup in historic nodes
func (suite *Suite) TestGetTransactionResultReturnsValidTransactionResultFromHistoricNode() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		coll := flow.CollectionFromTransactions([]*flow.Transaction{&tx})

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.Anything).
			Return(&transactionResultResponse, nil).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator

		backend, err := New(params)
		suite.Require().NoError(err)

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)
	})
}

func (suite *Suite) withGetTransactionCachingTestSetup(f func(b *flow.Block, t *flow.Transaction)) {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		block := unittest.BlockFixture()
		tbody := unittest.TransactionBodyFixture()
		tx := unittest.TransactionFixture()
		tx.TransactionBody = tbody

		suite.transactions.
			On("ByID", tx.ID()).
			Return(nil, storage.ErrNotFound)

		suite.state.On("AtBlockID", block.ID()).Return(snap, nil).Once()

		f(&block, &tx)
	})
}

// TestGetTransactionResultFromCache get historic transaction result from cache
func (suite *Suite) TestGetTransactionResultFromCache() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		transactionResultResponse := access.TransactionResultResponse{
			Status:     entities.TransactionStatus_EXECUTED,
			StatusCode: uint32(entities.TransactionStatus_EXECUTED),
		}

		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(&transactionResultResponse, nil).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp.StatusCode)

		resp2, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusExecuted, resp2.Status)
		suite.Require().Equal(uint(flow.TransactionStatusExecuted), resp2.StatusCode)

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}

// TestGetTransactionResultCacheNonExistent tests caches non existing result
func (suite *Suite) TestGetTransactionResultCacheNonExistent() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
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
		res, ok := backend.txResultCache.Get(tx.ID())
		suite.Require().True(ok)
		suite.Require().Equal(res, &acc.TransactionResult{
			Status:     txStatus,
			StatusCode: uint(txStatus),
		})

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}

// TestGetTransactionResultUnknownFromCache retrieve unknown result from cache.
func (suite *Suite) TestGetTransactionResultUnknownFromCache() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		suite.historicalAccessClient.
			On("GetTransactionResult", mock.Anything, mock.AnythingOfType("*access.GetTransactionRequest")).
			Return(nil, status.Errorf(codes.NotFound, "no known transaction with ID %s", tx.ID())).Once()

		params := suite.defaultBackendParams()
		params.HistoricalAccessNodes = []access.AccessAPIClient{suite.historicalAccessClient}
		params.Communicator = suite.communicator
		params.TxResultCacheSize = 10

		backend, err := New(params)
		suite.Require().NoError(err)

		coll := flow.CollectionFromTransactions([]*flow.Transaction{tx})

		resp, err := backend.GetTransactionResult(
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
		res, ok := backend.txResultCache.Get(tx.ID())
		suite.Require().True(ok)
		suite.Require().Equal(res, &acc.TransactionResult{
			Status:     txStatus,
			StatusCode: uint(txStatus),
		})

		resp2, err := backend.GetTransactionResult(
			context.Background(),
			tx.ID(),
			block.ID(),
			coll.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(flow.TransactionStatusUnknown, resp2.Status)
		suite.Require().Equal(uint(flow.TransactionStatusUnknown), resp2.StatusCode)

		suite.historicalAccessClient.AssertExpectations(suite.T())
	})
}

// TestLookupTransactionErrorMessageByTransactionID_HappyPath verifies the lookup of a transaction error message
// by block id and transaction id.
// It tests two cases:
// 1. Happy path where the error message is fetched from the EN if it's not found in the cache.
// 2. Happy path where the error message is served from the storage database if it exists.
func (suite *Suite) TestLookupTransactionErrorMessageByTransactionID_HappyPath() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	// Setup mock receipts and execution node identities.
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	params := suite.defaultBackendParams()
	params.TxResultErrorMessages = suite.txErrorMessages

	// Test case: transaction error message is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		params.ConnFactory = suite.setupConnectionFactory()
		params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

		// Mock the cache lookup for the transaction error message, returning "not found".
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()

		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the execution node API call to fetch the error message.
		exeEventReq := &execproto.GetTransactionErrorMessageRequest{
			BlockId:       blockId[:],
			TransactionId: failedTxId[:],
		}
		exeEventResp := &execproto.GetTransactionErrorMessageResponse{
			TransactionId: failedTxId[:],
			ErrorMessage:  expectedErrorMsg,
		}
		suite.execClient.On("GetTransactionErrorMessage", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly.
		errMsg, err := backend.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})

	// Test case: transaction error message is fetched from the storage database.
	suite.Run("happy path from storage db", func() {
		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the cache lookup for the transaction error message, returning a stored result.
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(&flow.TransactionResultErrorMessage{
				TransactionID: failedTxId,
				ErrorMessage:  expectedErrorMsg,
				Index:         failedTxIndex,
				ExecutorID:    unittest.IdentifierFixture(),
			}, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly from storage.
		errMsg, err := backend.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})
}

// TestLookupTransactionErrorMessageByTransactionID_FailedToFetch tests the case when a transaction error message
// is not in the cache and needs to be fetched from the EN, but the EN fails to return it.
// It tests three cases:
// 1. The transaction is not found in the transaction results, leading to a "NotFound" error.
// 2. The transaction result is not failed, and the error message is empty.
// 3. The transaction result is failed, and the error message "failed" are returned.
func (suite *Suite) TestLookupTransactionErrorMessageByTransactionID_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	// Setup mock receipts and execution node identities.
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// Create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	params := suite.defaultBackendParams()
	// The connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	// Initialize the transaction results index with the mock reporter.
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

	backend, err := New(params)
	suite.Require().NoError(err)

	// Test case: failed to fetch from EN, transaction is unknown.
	suite.Run("failed to fetch from EN, unknown tx", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction and tx error message is not found in the storage.
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()
		suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()

		// Perform the lookup and expect a "NotFound" error with an empty error message.
		errMsg, err := backend.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is not failed.
	suite.Run("failed to fetch from EN, tx result is not failed", func() {
		// Lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error message is not found in storage.
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction result exists and is not failed.
		suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(&flow.LightTransactionResult{
				TransactionID:   failedTxId,
				Failed:          false,
				ComputationUsed: 0,
			}, nil).Once()

		// Perform the lookup and expect no error and an empty error message.
		errMsg, err := backend.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is failed.
	suite.Run("failed to fetch from EN, tx result is failed", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error message is not found in storage.
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction result exists and is failed.
		suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(&flow.LightTransactionResult{
				TransactionID:   failedTxId,
				Failed:          true,
				ComputationUsed: 0,
			}, nil).Once()

		// Perform the lookup and expect the failed error message to be returned.
		errMsg, err := backend.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Equal(errMsg, FailedErrorMessage)
		suite.assertAllExpectations()
	})
}

// TestLookupTransactionErrorMessageByIndex_HappyPath verifies the lookup of a transaction error message
// by block ID and transaction index.
// It tests two cases:
// 1. Happy path where the error message is fetched from the EN if it is not found in the cache.
// 2. Happy path where the error message is served from the storage database if it exists.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_HappyPath() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	// Setup mock receipts and execution node identities.
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	params := suite.defaultBackendParams()
	params.TxResultErrorMessages = suite.txErrorMessages

	// Test case: transaction error message is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		params.ConnFactory = suite.setupConnectionFactory()
		params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

		// Mock the cache lookup for the transaction error message, returning "not found".
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()

		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the execution node API call to fetch the error message.
		exeEventReq := &execproto.GetTransactionErrorMessageByIndexRequest{
			BlockId: blockId[:],
			Index:   failedTxIndex,
		}
		exeEventResp := &execproto.GetTransactionErrorMessageResponse{
			TransactionId: failedTxId[:],
			ErrorMessage:  expectedErrorMsg,
		}
		suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly.
		errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})

	// Test case: transaction error message is fetched from the storage database.
	suite.Run("happy path from storage db", func() {
		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the cache lookup for the transaction error message, returning a stored result.
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(&flow.TransactionResultErrorMessage{
				TransactionID: failedTxId,
				ErrorMessage:  expectedErrorMsg,
				Index:         failedTxIndex,
				ExecutorID:    unittest.IdentifierFixture(),
			}, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly from storage.
		errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})
}

// TestLookupTransactionErrorMessageByIndex_FailedToFetch verifies the behavior of looking up a transaction error message by index
// when the error message is not in the cache, and fetching it from the EN fails.
// It tests three cases:
// 1. The transaction is not found in the transaction results, leading to a "NotFound" error.
// 2. The transaction result is not failed, and the error message is empty.
// 3. The transaction result is failed, and the error message "failed" are returned.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTxIndex := rand.Uint32()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	// Setup mock receipts and execution node identities.
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// Create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// Create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	// Initialize the transaction results index with the mock reporter.
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

	backend, err := New(params)
	suite.Require().NoError(err)

	// Test case: failed to fetch from EN, transaction is unknown.
	suite.Run("failed to fetch from EN, unknown tx", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction and tx error message is not found in the storage.
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()
		suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()

		// Perform the lookup and expect a "NotFound" error with an empty error message.
		errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is not failed.
	suite.Run("failed to fetch from EN, tx result is not failed", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error message is not found in storage.
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction result exists and is not failed.
		suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(&flow.LightTransactionResult{
				TransactionID:   failedTxId,
				Failed:          false,
				ComputationUsed: 0,
			}, nil).Once()

		// Perform the lookup and expect no error and an empty error message.
		errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is failed.
	suite.Run("failed to fetch from EN, tx result is failed", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error message is not found in storage.
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction result exists and is failed.
		suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(&flow.LightTransactionResult{
				TransactionID:   failedTxId,
				Failed:          true,
				ComputationUsed: 0,
			}, nil).Once()

		// Perform the lookup and expect the failed error message to be returned.
		errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(errMsg, FailedErrorMessage)
		suite.assertAllExpectations()
	})
}

// TestLookupTransactionErrorMessagesByBlockID_HappyPath verifies the lookup of transaction error messages by block ID.
// It tests two cases:
// 1. Happy path where the error messages are fetched from the EN if they are not found in the cache.
// 2. Happy path where the error messages are served from the storage database if they exist.
func (suite *Suite) TestLookupTransactionErrorMessagesByBlockID_HappyPath() {
	block := unittest.BlockFixture()
	blockId := block.ID()

	resultsByBlockID := make([]flow.LightTransactionResult, 0)
	for i := 0; i < 5; i++ {
		resultsByBlockID = append(resultsByBlockID, flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0, // create a mix of failed and non-failed transactions
			ComputationUsed: 0,
		})
	}

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	params := suite.defaultBackendParams()
	params.TxResultErrorMessages = suite.txErrorMessages

	// Test case: transaction error messages is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		params.ConnFactory = suite.setupConnectionFactory()
		params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

		// Mock the cache lookup for the transaction error messages, returning "not found".
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()

		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the execution node API call to fetch the error messages.
		exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
			BlockId: blockId[:],
		}
		exeErrMessagesResp := &execproto.GetTransactionErrorMessagesResponse{}
		for _, result := range resultsByBlockID {
			r := result
			if r.Failed {
				errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, r.TransactionID)
				exeErrMessagesResp.Results = append(exeErrMessagesResp.Results, &execproto.GetTransactionErrorMessagesResponse_Result{
					TransactionId: r.TransactionID[:],
					ErrorMessage:  errMsg,
				})
			}
		}
		suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(exeErrMessagesResp, nil).
			Once()

		// Perform the lookup and assert that the error message is retrieved correctly.
		errMessages, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
		suite.Require().NoError(err)
		suite.Require().Len(errMessages, len(exeErrMessagesResp.Results))
		for _, expectedResult := range exeErrMessagesResp.Results {
			errMsg, ok := errMessages[convert.MessageToIdentifier(expectedResult.TransactionId)]
			suite.Require().True(ok)
			suite.Assert().Equal(expectedResult.ErrorMessage, errMsg)
		}
		suite.assertAllExpectations()
	})

	// Test case: transaction error messages is fetched from the storage database.
	suite.Run("happy path from storage db", func() {
		backend, err := New(params)
		suite.Require().NoError(err)

		// Mock the cache lookup for the transaction error messages, returning a stored result.
		var txErrorMessages []flow.TransactionResultErrorMessage
		for i, result := range resultsByBlockID {
			if result.Failed {
				errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, result.TransactionID)

				txErrorMessages = append(txErrorMessages,
					flow.TransactionResultErrorMessage{
						TransactionID: result.TransactionID,
						ErrorMessage:  errMsg,
						Index:         uint32(i),
						ExecutorID:    unittest.IdentifierFixture(),
					})
			}
		}
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(txErrorMessages, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly from storage.
		errMessages, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
		suite.Require().NoError(err)
		suite.Require().Len(errMessages, len(txErrorMessages))
		for _, expected := range txErrorMessages {
			errMsg, ok := errMessages[expected.TransactionID]
			suite.Require().True(ok)
			suite.Assert().Equal(expected.ErrorMessage, errMsg)
		}
		suite.assertAllExpectations()
	})
}

// TestLookupTransactionErrorMessagesByBlockID_FailedToFetch tests lookup of a transaction error messages by block ID,
// when a transaction result is not in the cache and needs to be fetched from EN, but the EN fails to return it.
// It tests three cases:
// 1. The transaction is not found in the transaction results, leading to a "NotFound" error.
// 2. The transaction result is not failed, and the error message is empty.
// 3. The transaction result is failed, and the error message "failed" are returned.
func (suite *Suite) TestLookupTransactionErrorMessagesByBlockID_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()

	// Setup mock receipts and execution node identities.
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// Create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	// Initialize the transaction results index with the mock reporter.
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

	backend, err := New(params)
	suite.Require().NoError(err)

	// Test case: failed to fetch from EN, transaction is unknown.
	suite.Run("failed to fetch from EN, unknown tx", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction and tx error messages is not found in the storage.
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()
		suite.transactionResults.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()

		// Perform the lookup and expect a "NotFound" error with an empty error message.
		errMsg, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
		suite.Require().Error(err)
		suite.Require().Equal(codes.NotFound, status.Code(err))
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is not failed.
	suite.Run("failed to fetch from EN, tx result is not failed", func() {
		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error message is not found in storage.
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction results exists and is not failed.
		suite.transactionResults.On("ByBlockID", blockId).
			Return([]flow.LightTransactionResult{
				{
					TransactionID:   unittest.IdentifierFixture(),
					Failed:          false,
					ComputationUsed: 0,
				},
				{
					TransactionID:   unittest.IdentifierFixture(),
					Failed:          false,
					ComputationUsed: 0,
				},
			}, nil).Once()

		// Perform the lookup and expect no error and an empty error messages.
		errMsg, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
		suite.Require().NoError(err)
		suite.Require().Empty(errMsg)
		suite.assertAllExpectations()
	})

	// Test case: failed to fetch from EN, but the transaction result is failed.
	suite.Run("failed to fetch from EN, tx result is failed", func() {
		failedResultsByBlockID := []flow.LightTransactionResult{
			{
				TransactionID:   unittest.IdentifierFixture(),
				Failed:          true,
				ComputationUsed: 0,
			},
			{
				TransactionID:   unittest.IdentifierFixture(),
				Failed:          true,
				ComputationUsed: 0,
			},
		}

		// lookup should try each of the 2 ENs in fixedENIDs
		suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).Return(nil,
			status.Error(codes.Unavailable, "")).Twice()

		// Setup mock that the transaction error messages is not found in storage.
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()

		// Setup mock that the transaction results exists and is failed.
		suite.transactionResults.On("ByBlockID", blockId).
			Return(failedResultsByBlockID, nil).Once()

		// Setup mock expected the transaction error messages after retrieving the failed result.
		expectedTxErrorMessages := make(map[flow.Identifier]string)
		for _, result := range failedResultsByBlockID {
			if result.Failed {
				expectedTxErrorMessages[result.TransactionID] = FailedErrorMessage
			}
		}

		// Perform the lookup and expect the failed error messages to be returned.
		errMsg, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
		suite.Require().NoError(err)
		suite.Require().Len(errMsg, len(expectedTxErrorMessages))
		for txID, expectedMessage := range expectedTxErrorMessages {
			actualMessage, ok := errMsg[txID]
			suite.Require().True(ok)
			suite.Assert().Equal(expectedMessage, actualMessage)
		}
		suite.assertAllExpectations()
	})
}

// TestGetSystemTransaction_HappyPath tests that GetSystemTransaction call returns system chunk transaction.
func (suite *Suite) TestGetSystemTransaction_HappyPath() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		suite.state.On("Sealed").Return(snap, nil).Maybe()

		params := suite.defaultBackendParams()
		backend, err := New(params)
		suite.Require().NoError(err)

		block := unittest.BlockFixture()
		blockID := block.ID()

		// Make the call for the system chunk transaction
		res, err := backend.GetSystemTransaction(context.Background(), blockID)
		suite.Require().NoError(err)
		// Expected system chunk transaction
		systemTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
		suite.Require().NoError(err)

		suite.Require().Equal(systemTx, res)
	})
}

// TestGetSystemTransactionResult_HappyPath tests that GetSystemTransactionResult call returns system transaction
// result for required block id.
func (suite *Suite) TestGetSystemTransactionResult_HappyPath() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		suite.state.On("Sealed").Return(snap, nil).Maybe()
		lastBlock, err := snap.Head()
		suite.Require().NoError(err)
		identities, err := snap.Identities(filter.Any)
		suite.Require().NoError(err)

		block := unittest.BlockWithParentFixture(lastBlock)
		blockID := block.ID()
		suite.state.On("AtBlockID", blockID).Return(
			unittest.StateSnapshotForKnownBlock(block.Header, identities.Lookup()), nil).Once()

		// block storage returns the corresponding block
		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		receipt1 := unittest.ReceiptForBlockFixture(block)
		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(flow.ExecutionReceiptList{receipt1}, nil)

		// the connection factory should be used to get the execution node client
		params := suite.defaultBackendParams()
		params.ConnFactory = suite.setupConnectionFactory()

		exeEventReq := &execproto.GetTransactionsByBlockIDRequest{
			BlockId: blockID[:],
		}

		// Generating events with event generator
		exeNodeEventEncodingVersion := entities.EventEncodingVersion_CCF_V0
		events := generator.GetEventsWithEncoding(1, exeNodeEventEncodingVersion)
		eventMessages := convert.EventsToMessages(events)

		exeEventResp := &execproto.GetTransactionResultsResponse{
			TransactionResults: []*execproto.GetTransactionResultResponse{{
				Events:               eventMessages,
				EventEncodingVersion: exeNodeEventEncodingVersion,
			}},
			EventEncodingVersion: exeNodeEventEncodingVersion,
		}

		suite.execClient.
			On("GetTransactionResultsByBlockID", mock.Anything, exeEventReq).
			Return(exeEventResp, nil).
			Once()

		backend, err := New(params)
		suite.Require().NoError(err)

		suite.execClient.
			On("GetTransactionResult", mock.Anything, mock.AnythingOfType("*execution.GetTransactionResultRequest")).
			Return(exeEventResp.TransactionResults[0], nil).
			Once()

		// Make the call for the system transaction result
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

		events, err = convert.MessagesToEventsWithEncodingConversion(eventMessages,
			exeNodeEventEncodingVersion,
			entities.EventEncodingVersion_JSON_CDC_V0)
		suite.Require().NoError(err)
		suite.Require().Equal(events, res.Events)
	})
}

func (suite *Suite) TestGetSystemTransactionResultFromStorage() {
	// Create fixtures for block, transaction, and collection
	block := unittest.BlockFixture()
	sysTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
	suite.Require().NoError(err)
	suite.Require().NotNil(sysTx)
	transaction := flow.Transaction{TransactionBody: *sysTx}
	txId := suite.systemTx.ID()
	blockId := block.ID()

	// Mock the behavior of the blocks and transactionResults objects
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil).
		Once()

	lightTxShouldFail := false
	suite.transactionResults.
		On("ByBlockIDTransactionID", blockId, txId).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          lightTxShouldFail,
			ComputationUsed: 0,
		}, nil).
		Once()

	suite.transactions.
		On("ByID", txId).
		Return(&transaction.TransactionBody, nil).
		Once()

	// Set up the events storage mock
	var eventsForTx []flow.Event
	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil).Once()

	// create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	indexReporter := index.NewReporter()
	err = indexReporter.Initialize(reporter)
	suite.Require().NoError(err)

	// Set up the backend parameters and the backend instance
	params := suite.defaultBackendParams()
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	params.EventsIndex = index.NewEventsIndex(indexReporter, suite.events)
	params.TxResultsIndex = index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

	backend, err := New(params)
	suite.Require().NoError(err)

	response, err := backend.GetSystemTransactionResult(context.Background(), blockId, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, block, txId, lightTxShouldFail, eventsForTx)
}

// TestGetSystemTransactionResult_BlockNotFound tests GetSystemTransactionResult function when block was not found.
func (suite *Suite) TestGetSystemTransactionResult_BlockNotFound() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		suite.state.On("Sealed").Return(snap, nil).Maybe()
		lastBlock, err := snap.Head()
		suite.Require().NoError(err)
		identities, err := snap.Identities(filter.Any)
		suite.Require().NoError(err)

		block := unittest.BlockWithParentFixture(lastBlock)
		blockID := block.ID()
		suite.state.On("AtBlockID", blockID).Return(
			unittest.StateSnapshotForKnownBlock(block.Header, identities.Lookup()), nil).Once()

		// block storage returns the ErrNotFound error
		suite.blocks.
			On("ByID", blockID).
			Return(nil, storage.ErrNotFound).
			Once()

		receipt1 := unittest.ReceiptForBlockFixture(block)
		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(flow.ExecutionReceiptList{receipt1}, nil)

		params := suite.defaultBackendParams()

		backend, err := New(params)
		suite.Require().NoError(err)

		// Make the call for the system transaction result
		res, err := backend.GetSystemTransactionResult(
			context.Background(),
			block.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)

		suite.Require().Nil(res)
		suite.Require().Error(err)
		suite.Require().Equal(err, status.Errorf(codes.NotFound, "not found: %v", fmt.Errorf("key not found")))
	})
}

// TestGetSystemTransactionResult_FailedEncodingConversion tests the GetSystemTransactionResult function with different
// event encoding versions.
func (suite *Suite) TestGetSystemTransactionResult_FailedEncodingConversion() {
	suite.withPreConfiguredState(func(snap protocol.Snapshot) {
		suite.state.On("Sealed").Return(snap, nil).Maybe()
		lastBlock, err := snap.Head()
		suite.Require().NoError(err)
		identities, err := snap.Identities(filter.Any)
		suite.Require().NoError(err)

		block := unittest.BlockWithParentFixture(lastBlock)
		blockID := block.ID()
		suite.state.On("AtBlockID", blockID).Return(
			unittest.StateSnapshotForKnownBlock(block.Header, identities.Lookup()), nil).Once()

		// block storage returns the corresponding block
		suite.blocks.
			On("ByID", blockID).
			Return(block, nil).
			Once()

		receipt1 := unittest.ReceiptForBlockFixture(block)
		suite.receipts.
			On("ByBlockID", block.ID()).
			Return(flow.ExecutionReceiptList{receipt1}, nil)

		// the connection factory should be used to get the execution node client
		params := suite.defaultBackendParams()
		params.ConnFactory = suite.setupConnectionFactory()

		exeEventReq := &execproto.GetTransactionsByBlockIDRequest{
			BlockId: blockID[:],
		}

		// create empty events
		eventsPerBlock := 10
		eventMessages := make([]*entities.Event, eventsPerBlock)

		exeEventResp := &execproto.GetTransactionResultsResponse{
			TransactionResults: []*execproto.GetTransactionResultResponse{{
				Events:               eventMessages,
				EventEncodingVersion: entities.EventEncodingVersion_JSON_CDC_V0,
			}},
		}

		suite.execClient.
			On("GetTransactionResultsByBlockID", mock.Anything, exeEventReq).
			Return(exeEventResp, nil).
			Once()

		backend, err := New(params)
		suite.Require().NoError(err)

		suite.execClient.
			On("GetTransactionResult", mock.Anything, mock.AnythingOfType("*execution.GetTransactionResultRequest")).
			Return(exeEventResp.TransactionResults[0], nil).
			Once()

		// Make the call for the system transaction result
		res, err := backend.GetSystemTransactionResult(
			context.Background(),
			block.ID(),
			entities.EventEncodingVersion_CCF_V0,
		)

		suite.Require().Nil(res)
		suite.Require().Error(err)
		suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to convert events to message: %v",
			fmt.Errorf("conversion from format JSON_CDC_V0 to CCF_V0 is not supported")))
	})
}

func (suite *Suite) assertTransactionResultResponse(
	err error,
	response *acc.TransactionResult,
	block flow.Block,
	txId flow.Identifier,
	txFailed bool,
	eventsForTx []flow.Event,
) {
	suite.Require().NoError(err)
	suite.Assert().Equal(block.ID(), response.BlockID)
	suite.Assert().Equal(block.Header.Height, response.BlockHeight)
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

// TestTransactionResultFromStorage tests the retrieval of a transaction result (flow.TransactionResult) from storage
// instead of requesting it from the Execution Node.
func (suite *Suite) TestTransactionResultFromStorage() {
	// Create fixtures for block, transaction, and collection
	block := unittest.BlockFixture()
	transaction := unittest.TransactionFixture()
	col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := col.Guarantee()
	block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
	txId := transaction.ID()
	blockId := block.ID()

	// Mock the behavior of the blocks and transactionResults objects
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	suite.transactionResults.On("ByBlockIDTransactionID", blockId, txId).
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
	suite.collections.On("LightByID", col.ID()).Return(&lightCol, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents, flow.EventAccountCreated)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	indexReporter := index.NewReporter()
	err := indexReporter.Initialize(reporter)
	suite.Require().NoError(err)

	// Set up the backend parameters and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	params.EventsIndex = index.NewEventsIndex(indexReporter, suite.events)
	params.TxResultsIndex = index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

	backend, err := New(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response

	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       blockId[:],
		TransactionId: txId[:],
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: txId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	response, err := backend.GetTransactionResult(context.Background(), txId, blockId, flow.ZeroID, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, block, txId, true, eventsForTx)
}

// TestTransactionByIndexFromStorage tests the retrieval of a transaction result (flow.TransactionResult) by index
// and returns it from storage instead of requesting from the Execution Node.
func (suite *Suite) TestTransactionByIndexFromStorage() {
	// Create fixtures for block, transaction, and collection
	block := unittest.BlockFixture()
	transaction := unittest.TransactionFixture()
	col := flow.CollectionFromTransactions([]*flow.Transaction{&transaction})
	guarantee := col.Guarantee()
	block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
	blockId := block.ID()
	txId := transaction.ID()
	txIndex := rand.Uint32()

	// Set up the light collection and mock the behavior of the collections object
	lightCol := col.Light()
	suite.collections.On("LightByID", col.ID()).Return(&lightCol, nil)

	// Mock the behavior of the blocks and transactionResults objects
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, txIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents, flow.EventAccountCreated)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionIndex", blockId, txIndex).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	indexReporter := index.NewReporter()
	err := indexReporter.Initialize(reporter)
	suite.Require().NoError(err)

	// Set up the backend parameters and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	params.EventsIndex = index.NewEventsIndex(indexReporter, suite.events)
	params.TxResultsIndex = index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

	backend, err := New(params)
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

	suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	response, err := backend.GetTransactionResultByIndex(context.Background(), blockId, txIndex, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.assertTransactionResultResponse(err, response, block, txId, true, eventsForTx)
}

// TestTransactionResultsByBlockIDFromStorage tests the retrieval of transaction results ([]flow.TransactionResult)
// by block ID from storage instead of requesting from the Execution Node.
func (suite *Suite) TestTransactionResultsByBlockIDFromStorage() {
	// Create fixtures for the block and collection
	block := unittest.BlockFixture()
	col := unittest.CollectionFixture(2)
	guarantee := col.Guarantee()
	block.SetPayload(unittest.PayloadFixture(unittest.WithGuarantees(&guarantee)))
	blockId := block.ID()

	// Mock the behavior of the blocks, collections and light transaction results objects
	suite.blocks.
		On("ByID", blockId).
		Return(&block, nil)
	lightCol := col.Light()
	suite.collections.On("LightByID", mock.Anything).Return(&lightCol, nil)

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
	suite.transactionResults.On("ByBlockID", blockId).Return(lightTxResults, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := unittest.EventsFixture(totalEvents, flow.EventAccountCreated)
	eventMessages := make([]*entities.Event, totalEvents)
	for j, event := range eventsForTx {
		eventMessages[j] = convert.EventToMessage(event)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, mock.Anything).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	indexReporter := index.NewReporter()
	err := indexReporter.Initialize(reporter)
	suite.Require().NoError(err)

	// Set up the state and snapshot mocks and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	params.EventsIndex = index.NewEventsIndex(indexReporter, suite.events)
	params.TxResultsIndex = index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

	params.TxResultQueryMode = IndexQueryModeLocalOnly

	backend, err := New(params)
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

	suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	response, err := backend.GetTransactionResultsByBlockID(context.Background(), blockId, entities.EventEncodingVersion_JSON_CDC_V0)
	suite.Require().NoError(err)
	suite.Assert().Equal(len(lightTxResults), len(response))

	// Assertions for each transaction result in the response
	for i, responseResult := range response {
		lightTx := lightTxResults[i]
		suite.assertTransactionResultResponse(err, responseResult, block, lightTx.TransactionID, lightTx.Failed, eventsForTx)
	}
}
