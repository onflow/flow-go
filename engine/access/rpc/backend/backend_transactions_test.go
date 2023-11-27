package backend

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	acc "github.com/onflow/flow-go/access"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func (suite *Suite) withPreConfiguredState(f func(snap protocol.Snapshot)) {
	identities := unittest.CompleteIdentitySet()
	rootSnapshot := unittest.RootSnapshotFixture(identities)
	util.RunWithFullProtocolState(suite.T(), rootSnapshot, func(db *badger.DB, state *bprotocol.ParticipantState) {
		epochBuilder := unittest.NewEpochBuilder(suite.T(), state)

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

		snap := state.AtHeight(epoch1.Range()[0])
		suite.state.On("Final").Return(snap).Once()
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
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
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
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
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

// TestGetTransactionResultUnknownFromCache retrive unknown result from cache
func (suite *Suite) TestGetTransactionResultUnknownFromCache() {
	suite.withGetTransactionCachingTestSetup(func(block *flow.Block, tx *flow.Transaction) {
		suite.historicalAccessClient.
			On("GetTransactionResult", mock.AnythingOfType("*context.emptyCtx"), mock.AnythingOfType("*access.GetTransactionRequest")).
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

// TestLookupTransactionErrorMessage_HappyPath tests lookup of a transaction error message. In a happy path if it wasn't found in the cache, it
// has to be fetched from the execution node, otherwise served from the cache.
// If the transaction has not failed, the error message must be empty.
func (suite *Suite) TestLookupTransactionErrorMessage_HappyPath() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	nonFailedTx := unittest.TransactionFixture()
	nonFailedTxID := nonFailedTx.ID()

	suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).Once()

	suite.transactionResults.On("ByBlockIDTransactionID", blockId, nonFailedTxID).
		Return(&flow.LightTransactionResult{
			TransactionID:   nonFailedTxID,
			Failed:          false,
			ComputationUsed: 0,
		}, nil).Once()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	errMsg, err := backend.lookupTransactionErrorMessage(context.Background(), blockId, nonFailedTxID)
	suite.Require().NoError(err)
	suite.Require().Empty(errMsg)

	expectedErrorMsg := "some error"

	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       blockId[:],
		TransactionId: failedTxId[:],
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	errMsg, err = backend.lookupTransactionErrorMessage(context.Background(), blockId, failedTxId)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMsg, err = backend.lookupTransactionErrorMessage(context.Background(), blockId, failedTxId)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessage_UnknownTransaction tests lookup of a transaction error message, when a transaction result
// has not been synced yet, in this case nothing we can do but return an error.
func (suite *Suite) TestLookupTransactionErrorMessage_UnknownTransaction() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
		Return(nil, storage.ErrNotFound).Once()

	params := suite.defaultBackendParams()
	backend, err := New(params)
	suite.Require().NoError(err)

	errMsg, err := backend.lookupTransactionErrorMessage(context.Background(), blockId, failedTxId)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessage_FailedToFetch tests lookup of a transaction error message, when a transaction result
// is not in the cache and needs to be fetched from EN, but the EN fails to return it.
func (suite *Suite) TestLookupTransactionErrorMessage_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.transactionResults.On("ByBlockIDTransactionID", blockId, failedTxId).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).Once()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.Unavailable, "")).Twice()

	errMsg, err := backend.lookupTransactionErrorMessage(context.Background(), blockId, failedTxId)
	suite.Require().Error(err)
	suite.Require().Equal(codes.Unavailable, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessageByIndex_HappyPath tests lookup of a transaction error message by index.
// In a happy path if it wasn't found in the cache, it has to be fetched from the execution node, otherwise served from the cache.
// If the transaction has not failed, the error message must be empty.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_HappyPath() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	nonFailedTx := unittest.TransactionFixture()
	nonFailedTxID := nonFailedTx.ID()
	nonFailedTxIndex := rand.Uint32()

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).Twice()

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, nonFailedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   nonFailedTxID,
			Failed:          false,
			ComputationUsed: 0,
		}, nil).Once()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	errMsg, err := backend.lookupTransactionErrorMessageByIndex(context.Background(), blockId, nonFailedTxIndex)
	suite.Require().NoError(err)
	suite.Require().Empty(errMsg)

	expectedErrorMsg := "some error"

	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       blockId[:],
		TransactionId: failedTxId[:],
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	errMsg, err = backend.lookupTransactionErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMsg, err = backend.lookupTransactionErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessageByIndex_UnknownTransaction tests lookup of a transaction error message by index,
// when a transaction result has not been synced yet, in this case nothing we can do but return an error.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_UnknownTransaction() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTxIndex := rand.Uint32()

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
		Return(nil, storage.ErrNotFound).Once()

	params := suite.defaultBackendParams()
	backend, err := New(params)
	suite.Require().NoError(err)

	errMsg, err := backend.lookupTransactionErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessageByIndex_FailedToFetch tests lookup of a transaction error message by index,
// when a transaction result is not in the cache and needs to be fetched from EN, but the EN fails to return it.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).Once()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.Unavailable, "")).Twice()

	errMsg, err := backend.lookupTransactionErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
	suite.Require().Error(err)
	suite.Require().Equal(codes.Unavailable, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessages_HappyPath tests lookup of a transaction error messages by block ID.
// In a happy path, it has to be fetched from the execution node if there are no cached results.
// All fetched transactions have to be cached for future calls.
func (suite *Suite) TestLookupTransactionErrorMessages_HappyPath() {
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

	suite.transactionResults.On("ByBlockID", blockId).
		Return(resultsByBlockID, nil).Twice()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	expectedErrorMsg := "some error"

	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}

	exeEventResp := &execproto.GetTransactionErrorMessagesResponse{}
	for _, result := range resultsByBlockID {
		r := result
		if r.Failed {
			exeEventResp.Results = append(exeEventResp.Results, &execproto.GetTransactionErrorMessagesResponse_Result{
				TransactionId: r.TransactionID[:],
				ErrorMessage:  fmt.Sprintf("%s.%s", expectedErrorMsg, r.TransactionID),
			})
		}
	}

	suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	errMessages, err := backend.lookupTransactionErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().NoError(err)
	for _, expectedResult := range resultsByBlockID {
		if expectedResult.Failed {
			errMsg, ok := errMessages[expectedResult.TransactionID]
			suite.Require().True(ok)
			suite.Assert().Equal(fmt.Sprintf("%s.%s", expectedErrorMsg, expectedResult.TransactionID), errMsg)
		}
	}

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMessages, err = backend.lookupTransactionErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().NoError(err)
	for _, expectedResult := range resultsByBlockID {
		if expectedResult.Failed {
			errMsg, ok := errMessages[expectedResult.TransactionID]
			suite.Require().True(ok)
			suite.Assert().Equal(fmt.Sprintf("%s.%s", expectedErrorMsg, expectedResult.TransactionID), errMsg)
		}
	}
	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessages_HappyPath_NoFailedTxns tests lookup of a transaction error messages by block ID.
// In a happy path where a block with no failed txns is requested. We don't want to perform an RPC call in this case.
func (suite *Suite) TestLookupTransactionErrorMessages_HappyPath_NoFailedTxns() {
	block := unittest.BlockFixture()
	blockId := block.ID()

	resultsByBlockID := []flow.LightTransactionResult{
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
	}

	suite.transactionResults.On("ByBlockID", blockId).
		Return(resultsByBlockID, nil).Once()

	params := suite.defaultBackendParams()

	backend, err := New(params)
	suite.Require().NoError(err)

	errMessages, err := backend.lookupTransactionErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().NoError(err)
	suite.Require().Empty(errMessages)
	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessages_UnknownTransaction tests lookup of a transaction error messages by block ID,
// when a transaction results for block has not been synced yet, in this case nothing we can do but return an error.
func (suite *Suite) TestLookupTransactionErrorMessages_UnknownTransaction() {
	block := unittest.BlockFixture()
	blockId := block.ID()

	suite.transactionResults.On("ByBlockID", blockId).
		Return(nil, storage.ErrNotFound).Once()

	params := suite.defaultBackendParams()
	backend, err := New(params)
	suite.Require().NoError(err)

	errMsg, err := backend.lookupTransactionErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessages_FailedToFetch tests lookup of a transaction error messages by block ID,
// when a transaction result is not in the cache and needs to be fetched from EN, but the EN fails to return it.
func (suite *Suite) TestLookupTransactionErrorMessages_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()

	resultsByBlockID := []flow.LightTransactionResult{
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

	suite.transactionResults.On("ByBlockID", blockId).
		Return(resultsByBlockID, nil).Once()

	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()

	backend, err := New(params)
	suite.Require().NoError(err)

	// pretend the first transaction has been cached, but there are multiple failed txns so still a request has to be made.
	backend.txErrorMessagesCache.Add(resultsByBlockID[0].TransactionID, "some error")

	suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.Unavailable, "")).Twice()

	errMsg, err := backend.lookupTransactionErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().Error(err)
	suite.Require().Equal(codes.Unavailable, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
}
