package error_message_provider

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	communicatormock "github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator/mock"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

const expectedErrorMsg = "expected test error"

type Suite struct {
	suite.Suite

	log      zerolog.Logger
	state    *protocolmock.State
	snapshot *protocolmock.Snapshot

	receipts           *storagemock.ExecutionReceipts
	transactionResults *storagemock.LightTransactionResults
	txErrorMessages    *storagemock.TransactionResultErrorMessages

	execClient *accessmock.ExecutionAPIClient

	communicator *communicatormock.Communicator
	nodeProvider *commonrpc.ExecutionNodeIdentitiesProvider

	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {

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

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	// Test case: transaction error message is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		connFactory := suite.setupConnectionFactory()

		// Mock the cache lookup for the transaction error message, returning "not found".
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(nil, storage.ErrNotFound).Once()

		reporter := syncmock.NewIndexReporter(suite.T())
		//reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
		//reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)
		indexReporter := index.NewReporter()
		err := indexReporter.Initialize(reporter)
		suite.Require().NoError(err)
		txResultsIndex := index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

		suite.communicator.On("CallAvailableNode",
			mock.Anything,
			mock.Anything,
			mock.Anything).
			Return(nil).Once()

		errMessageProvider := NewTxErrorMessageProvider(
			suite.log,
			suite.txErrorMessages,
			txResultsIndex,
			connFactory,
			suite.communicator,
			suite.nodeProvider,
		)

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
		errMsg, err := errMessageProvider.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})

	// Test case: transaction error message is fetched from the storage database.
	suite.Run("happy path from storage db", func() {
		connFactory := suite.setupConnectionFactory()
		reporter := syncmock.NewIndexReporter(suite.T())
		//reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
		//reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

		indexReporter := index.NewReporter()
		err := indexReporter.Initialize(reporter)
		suite.Require().NoError(err)

		txResultsIndex := index.NewTransactionResultsIndex(indexReporter, suite.transactionResults)

		errMessageProvider := NewTxErrorMessageProvider(
			suite.log,
			suite.txErrorMessages,
			txResultsIndex,
			connFactory,
			suite.communicator,
			suite.nodeProvider,
		)

		// Mock the cache lookup for the transaction error message, returning a stored result.
		suite.txErrorMessages.On("ByBlockIDTransactionID", blockId, failedTxId).
			Return(&flow.TransactionResultErrorMessage{
				TransactionID: failedTxId,
				ErrorMessage:  expectedErrorMsg,
				Index:         failedTxIndex,
				ExecutorID:    unittest.IdentifierFixture(),
			}, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly from storage.
		errMsg, err := errMessageProvider.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
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

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// The connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	// Initialize the transaction results index with the mock reporter.
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

	errMessageProvider := NewTxErrorMessageProvider(
		params.Log,
		params.TxResultErrorMessages,
		params.TxResultsIndex,
		params.ConnFactory,
		params.Communicator,
		params.ExecNodeIdentitiesProvider,
	)

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
		errMsg, err := errMessageProvider.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
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
		errMsg, err := errMessageProvider.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
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
		errMsg, err := errMessageProvider.LookupErrorMessageByTransactionID(context.Background(), blockId, block.Header.Height, failedTxId)
		suite.Require().NoError(err)
		suite.Require().Equal(errMsg, DefaultFailedErrorMessage)
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

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	params.TxResultErrorMessages = suite.txErrorMessages

	// Test case: transaction error message is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		params.ConnFactory = suite.setupConnectionFactory()

		// Mock the cache lookup for the transaction error message, returning "not found".
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(nil, storage.ErrNotFound).Once()

		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

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
		errMsg, err := errMessageProvider.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
		suite.assertAllExpectations()
	})

	// Test case: transaction error message is fetched from the storage database.
	suite.Run("happy path from storage db", func() {
		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

		// Mock the cache lookup for the transaction error message, returning a stored result.
		suite.txErrorMessages.On("ByBlockIDTransactionIndex", blockId, failedTxIndex).
			Return(&flow.TransactionResultErrorMessage{
				TransactionID: failedTxId,
				ErrorMessage:  expectedErrorMsg,
				Index:         failedTxIndex,
				ExecutorID:    unittest.IdentifierFixture(),
			}, nil).Once()

		// Perform the lookup and assert that the error message is retrieved correctly from storage.
		errMsg, err := errMessageProvider.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
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
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)

	// Create a mock index reporter
	reporter := syncmock.NewIndexReporter(suite.T())
	reporter.On("LowestIndexedHeight").Return(block.Header.Height, nil)
	reporter.On("HighestIndexedHeight").Return(block.Header.Height+10, nil)

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = connFactory
	// Initialize the transaction results index with the mock reporter.
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

	errMessageProvider := NewTxErrorMessageProvider(
		params.Log,
		params.TxResultErrorMessages,
		params.TxResultsIndex,
		params.ConnFactory,
		params.Communicator,
		params.ExecNodeIdentitiesProvider,
	)

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
		errMsg, err := errMessageProvider.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
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
		errMsg, err := errMessageProvider.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
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
		errMsg, err := errMessageProvider.LookupErrorMessageByIndex(context.Background(), blockId, block.Header.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(errMsg, DefaultFailedErrorMessage)
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

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	params.TxResultErrorMessages = suite.txErrorMessages

	// Test case: transaction error messages is fetched from the EN.
	suite.Run("happy path from EN", func() {
		// the connection factory should be used to get the execution node client
		params.ConnFactory = suite.setupConnectionFactory()

		// Mock the cache lookup for the transaction error messages, returning "not found".
		suite.txErrorMessages.On("ByBlockID", blockId).
			Return(nil, storage.ErrNotFound).Once()

		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

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
		errMessages, err := errMessageProvider.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
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
		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

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
		errMessages, err := errMessageProvider.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
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

	suite.fixedExecutionNodeIDs = fixedENIDs.NodeIDs()

	params := suite.defaultBackendParams()
	// the connection factory should be used to get the execution node client
	params.ConnFactory = suite.setupConnectionFactory()
	// Initialize the transaction results index with the mock reporter.
	params.TxResultsIndex = index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := params.TxResultsIndex.Initialize(reporter)
	suite.Require().NoError(err)

	params.TxResultErrorMessages = suite.txErrorMessages

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

		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

		// Perform the lookup and expect a "NotFound" error with an empty error message.
		errMsg, err := errMessageProvider.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
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

		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)

		// Perform the lookup and expect no error and an empty error messages.
		errMsg, err := errMessageProvider.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
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
				expectedTxErrorMessages[result.TransactionID] = DefaultFailedErrorMessage
			}
		}

		errMessageProvider := NewTxErrorMessageProvider(
			params.Log,
			params.TxResultErrorMessages,
			params.TxResultsIndex,
			params.ConnFactory,
			params.Communicator,
			params.ExecNodeIdentitiesProvider,
		)
		// Perform the lookup and expect the failed error messages to be returned.
		errMsg, err := errMessageProvider.LookupErrorMessagesByBlockID(context.Background(), blockId, block.Header.Height)
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

func (suite *Suite) setupConnectionFactory() connection.ConnectionFactory {
	connFactory := connectionmock.NewConnectionFactory(suite.T())
	connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mocks.MockCloser{}, nil)
	return connFactory
}

func (suite *Suite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	//suite.blocks.AssertExpectations(suite.T())
	//suite.headers.AssertExpectations(suite.T())
	//suite.collections.AssertExpectations(suite.T())
	//suite.transactions.AssertExpectations(suite.T())
	suite.execClient.AssertExpectations(suite.T())
}
