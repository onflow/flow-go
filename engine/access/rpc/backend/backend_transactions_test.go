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
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/util"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/generator"
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

// TestGetTransactionResultUnknownFromCache retrieve unknown result from cache.
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

	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       blockId[:],
		TransactionId: failedTxId[:],
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	errMsg, err := backend.LookupErrorMessageByTransactionId(context.Background(), blockId, failedTxId)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMsg, err = backend.LookupErrorMessageByTransactionId(context.Background(), blockId, failedTxId)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
	suite.assertAllExpectations()
}

// TestLookupTransactionErrorMessage_FailedToFetch tests lookup of a transaction error message, when a transaction result
// is not in the cache and needs to be fetched from EN, but the EN fails to return it.
func (suite *Suite) TestLookupTransactionErrorMessage_FailedToFetch() {
	block := unittest.BlockFixture()
	blockId := block.ID()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

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

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.execClient.On("GetTransactionErrorMessage", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.Unavailable, "")).Twice()

	errMsg, err := backend.LookupErrorMessageByTransactionId(context.Background(), blockId, failedTxId)
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

	suite.transactionResults.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).Twice()

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

	exeEventReq := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: blockId[:],
		Index:   failedTxIndex,
	}

	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).Return(exeEventResp, nil).Once()

	errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMsg, err = backend.LookupErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
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

	errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
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

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.execClient.On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).Return(nil,
		status.Error(codes.Unavailable, "")).Twice()

	errMsg, err := backend.LookupErrorMessageByIndex(context.Background(), blockId, failedTxIndex)
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
			errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, r.TransactionID)
			exeEventResp.Results = append(exeEventResp.Results, &execproto.GetTransactionErrorMessagesResponse_Result{
				TransactionId: r.TransactionID[:],
				ErrorMessage:  errMsg,
			})
		}
	}

	suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	errMessages, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().NoError(err)
	suite.Require().Len(errMessages, len(exeEventResp.Results))
	for _, expectedResult := range exeEventResp.Results {
		errMsg, ok := errMessages[convert.MessageToIdentifier(expectedResult.TransactionId)]
		suite.Require().True(ok)
		suite.Assert().Equal(expectedResult.ErrorMessage, errMsg)
	}

	// ensure the transaction error message is cached after retrieval; we do this by mocking the grpc call
	// only once
	errMessages, err = backend.LookupErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().NoError(err)
	suite.Require().Len(errMessages, len(exeEventResp.Results))
	for _, expectedResult := range exeEventResp.Results {
		errMsg, ok := errMessages[convert.MessageToIdentifier(expectedResult.TransactionId)]
		suite.Require().True(ok)
		suite.Assert().Equal(expectedResult.ErrorMessage, errMsg)
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

	errMessages, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId)
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

	errMsg, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId)
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

	errMsg, err := backend.LookupErrorMessagesByBlockID(context.Background(), blockId)
	suite.Require().Error(err)
	suite.Require().Equal(codes.Unavailable, status.Code(err))
	suite.Require().Empty(errMsg)

	suite.assertAllExpectations()
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

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

		// the connection factory should be used to get the execution node client
		params := suite.defaultBackendParams()
		params.ConnFactory = connFactory

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

		// Make the call for the system transaction result
		res, err := backend.GetSystemTransactionResult(
			context.Background(),
			block.ID(),
			entities.EventEncodingVersion_JSON_CDC_V0,
		)
		suite.Require().NoError(err)

		// Expected system chunk transaction
		systemTx, err := blueprints.SystemChunkTransaction(suite.chainID.Chain())
		suite.Require().NoError(err)

		suite.Require().Equal(flow.TransactionStatusExecuted, res.Status)
		suite.Require().Equal(systemTx.ID(), res.TransactionID)

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

		// create a mock connection factory
		connFactory := connectionmock.NewConnectionFactory(suite.T())
		connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

		// the connection factory should be used to get the execution node client
		params := suite.defaultBackendParams()
		params.ConnFactory = connFactory

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

		// Make the call for the system transaction result
		res, err := backend.GetSystemTransactionResult(
			context.Background(),
			block.ID(),
			entities.EventEncodingVersion_CCF_V0,
		)

		suite.Require().Nil(res)
		suite.Require().Error(err)
		suite.Require().Equal(err, status.Errorf(codes.Internal, "failed to convert events from system tx result: %v",
			fmt.Errorf("conversion from format JSON_CDC_V0 to CCF_V0 is not supported")))
	})
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
	eventsForTx := make([]flow.Event, totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j := range eventsForTx {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture(), 0)
		eventsForTx[j] = e
		eventMessages[j] = convert.EventToMessage(e)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, txId).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// Set up the backend parameters and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.Events = suite.events
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	backend, err := New(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response
	expectedErrorMsg := "expected test error"
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
	suite.Require().NoError(err)
	suite.Assert().Equal(blockId, response.BlockID)
	suite.Assert().Equal(block.Header.Height, response.BlockHeight)
	suite.Assert().Equal(txId, response.TransactionID)
	suite.Assert().Equal(col.ID(), response.CollectionID)
	suite.Assert().Equal(len(eventsForTx), len(response.Events))
	suite.Assert().Equal(expectedErrorMsg, response.ErrorMessage)
	// When there are error messages occurred in the transaction, the status should be 1
	suite.Assert().Equal(uint(1), response.StatusCode)
	suite.Assert().Equal(flow.TransactionStatusSealed, response.Status)
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
	eventsForTx := make([]flow.Event, totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j := range eventsForTx {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture(), 0)
		eventsForTx[j] = e
		eventMessages[j] = convert.EventToMessage(e)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionIndex", blockId, txIndex).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// Create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// Set up the backend parameters and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.Events = suite.events
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	backend, err := New(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response
	expectedErrorMsg := "expected test error"
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
	suite.Require().NoError(err)
	suite.Assert().Equal(blockId, response.BlockID)
	suite.Assert().Equal(block.Header.Height, response.BlockHeight)
	suite.Assert().Equal(txId, response.TransactionID)
	suite.Assert().Equal(col.ID(), response.CollectionID)
	suite.Assert().Equal(len(eventsForTx), len(response.Events))
	suite.Assert().Equal(expectedErrorMsg, response.ErrorMessage)
	suite.Assert().Equal(uint(1), response.StatusCode) // When there are error message occurred in transaction, the status should be 1
	suite.Assert().Equal(flow.TransactionStatusSealed, response.Status)
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
	for i, txId := range lightCol.Transactions {
		lightTxResults[i] = flow.LightTransactionResult{
			TransactionID:   txId,
			Failed:          false,
			ComputationUsed: 0,
		}
	}

	// Mark the first transaction as failed
	lightTxResults[0].Failed = true
	suite.transactionResults.On("ByBlockID", blockId).Return(lightTxResults, nil)

	// Set up the events storage mock
	totalEvents := 5
	eventsForTx := make([]flow.Event, totalEvents)
	eventMessages := make([]*entities.Event, totalEvents)
	for j := range eventsForTx {
		e := unittest.EventFixture(flow.EventAccountCreated, uint32(j), uint32(j), unittest.IdentifierFixture(), 0)
		eventsForTx[j] = e
		eventMessages[j] = convert.EventToMessage(e)
	}

	// expect a call to lookup events by block ID and transaction ID
	suite.events.On("ByBlockIDTransactionID", blockId, mock.Anything).Return(eventsForTx, nil)

	// Set up the state and snapshot mocks
	_, fixedENIDs := suite.setupReceipts(&block)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.state.On("Sealed").Return(suite.snapshot, nil).Maybe()
	suite.snapshot.On("Identities", mock.Anything).Return(fixedENIDs, nil)
	suite.snapshot.On("Head", mock.Anything).Return(block.Header, nil)

	// create a mock connection factory
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

	// Set up the state and snapshot mocks and the backend instance
	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	params.FixedExecutionNodeIDs = fixedENIDs.NodeIDs().Strings()
	params.Events = suite.events
	params.TxResultQueryMode = IndexQueryModeLocalOnly

	backend, err := New(params)
	suite.Require().NoError(err)

	// Set up the expected error message for the execution node response
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}

	expectedErrorMsg := "expected test error"

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
		suite.Assert().Equal(blockId, responseResult.BlockID)
		suite.Assert().Equal(block.Header.Height, responseResult.BlockHeight)
		suite.Assert().Equal(lightTx.TransactionID, responseResult.TransactionID)
		suite.Assert().Equal(col.ID(), responseResult.CollectionID)
		suite.Assert().Equal(len(eventsForTx), len(responseResult.Events))

		// When there are error message occurred in transaction, the status should be 1
		if lightTx.Failed {
			suite.Assert().Equal(uint(1), responseResult.StatusCode)
			suite.Assert().Equal(expectedErrorMsg, responseResult.ErrorMessage)
		} else {
			suite.Assert().Equal(uint(0), responseResult.StatusCode)
			suite.Assert().Equal("", responseResult.ErrorMessage)
		}

		suite.Assert().Equal(flow.TransactionStatusSealed, responseResult.Status)
	}
}
