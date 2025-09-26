package error_messages

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
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
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

	block                     *flow.Block
	blockID                   flow.Identifier
	fixedExecutionNodes       flow.IdentityList
	fixedExecutionNodeIDs     flow.IdentifierList
	preferredExecutionNodeIDs flow.IdentifierList

	receipts              *storagemock.ExecutionReceipts
	lightTxResults        *storagemock.LightTransactionResults
	txResultErrorMessages *storagemock.TransactionResultErrorMessages

	executionAPIClient *accessmock.ExecutionAPIClient

	nodeCommunicator  *node_communicator.NodeCommunicator
	nodeProvider      *commonrpc.ExecutionNodeIdentitiesProvider
	connectionFactory *connectionmock.ConnectionFactory

	reporter       *syncmock.IndexReporter
	indexReporter  *index.Reporter
	txResultsIndex *index.TransactionResultsIndex
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = unittest.Logger()
	suite.snapshot = protocolmock.NewSnapshot(suite.T())

	header := unittest.BlockHeaderFixture()
	params := protocolmock.NewParams(suite.T())
	params.On("FinalizedRoot").Return(header, nil).Maybe()
	params.On("SporkID").Return(unittest.IdentifierFixture(), nil).Maybe()
	params.On("SporkRootBlockHeight").Return(header.Height, nil).Maybe()
	params.On("SealedRoot").Return(header, nil).Maybe()

	suite.state = protocolmock.NewState(suite.T())
	suite.state.On("Params").Return(params).Maybe()

	suite.receipts = storagemock.NewExecutionReceipts(suite.T())
	suite.lightTxResults = storagemock.NewLightTransactionResults(suite.T())
	suite.txResultErrorMessages = storagemock.NewTransactionResultErrorMessages(suite.T())
	suite.executionAPIClient = accessmock.NewExecutionAPIClient(suite.T())
	suite.connectionFactory = connectionmock.NewConnectionFactory(suite.T())
	suite.nodeCommunicator = node_communicator.NewNodeCommunicator(false)

	suite.block = unittest.BlockFixture()
	suite.blockID = suite.block.ID()
	_, suite.fixedExecutionNodes = suite.setupReceipts(suite.block)
	suite.fixedExecutionNodeIDs = suite.fixedExecutionNodes.NodeIDs()
	suite.preferredExecutionNodeIDs = nil

	suite.nodeProvider = commonrpc.NewExecutionNodeIdentitiesProvider(
		suite.log,
		suite.state,
		suite.receipts,
		suite.preferredExecutionNodeIDs,
		suite.fixedExecutionNodeIDs,
	)

	suite.reporter = syncmock.NewIndexReporter(suite.T())
	suite.indexReporter = index.NewReporter()
	err := suite.indexReporter.Initialize(suite.reporter)
	suite.Require().NoError(err)
	suite.txResultsIndex = index.NewTransactionResultsIndex(suite.indexReporter, suite.lightTxResults)
}

func (suite *Suite) TestLookupByTxID_FromExecutionNode_HappyPath() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	// Setup mock receipts and execution node identities.
	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()

	// the connection factory should be used to get the execution node client
	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	// Mock the cache lookup for the transaction error message, returning "not found".
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	// Mock the execution node API call to fetch the error message.
	exeEventReq := &execproto.GetTransactionErrorMessageRequest{
		BlockId:       suite.blockID[:],
		TransactionId: failedTxId[:],
	}
	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}

	suite.executionAPIClient.
		On("GetTransactionErrorMessage", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	// Perform the lookup and assert that the error message is retrieved correctly.
	errMsg, err := errMessageProvider.ErrorMessageByTransactionID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxId,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
}

func (suite *Suite) TestLookupByTxID_FromStorage_HappyPath() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	// Mock the cache lookup for the transaction error message, returning a stored result.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(&flow.TransactionResultErrorMessage{
			TransactionID: failedTxId,
			ErrorMessage:  expectedErrorMsg,
			Index:         failedTxIndex,
			ExecutorID:    unittest.IdentifierFixture(),
		}, nil).
		Once()

	errMsg, err := errMessageProvider.ErrorMessageByTransactionID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxId,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
}

func (suite *Suite) TestLookupByTxID_ExecNodeError_UnknownTx() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessage", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// Setup mock that the transaction and tx error message is not found in the storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(nil, storage.ErrNotFound).
		Once()
	suite.lightTxResults.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByTransactionID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxId,
	)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupByTxID_ExecNodeError_TxResultNotFailed() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	// Lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessage", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// Setup mock that the transaction error message is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(nil, storage.ErrNotFound).
		Once()

	// Setup mock that the transaction result exists and is not failed.
	suite.lightTxResults.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          false,
			ComputationUsed: 0,
		}, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByTransactionID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxId,
	)
	suite.Require().NoError(err)
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupByTxID_ExecNodeError_TxResultFailed() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessage", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// Setup mock that the transaction error message is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(nil, storage.ErrNotFound).
		Once()

	// Setup mock that the transaction result exists and is failed.
	suite.lightTxResults.
		On("ByBlockIDTransactionID", suite.blockID, failedTxId).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByTransactionID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxId,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(errMsg, DefaultFailedErrorMessage)
}

func (suite *Suite) TestLookupByIndex_FromExecutionNode_HappyPath() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.
		On("Identities", mock.Anything).
		Return(suite.fixedExecutionNodes, nil).
		Once()

	exeEventReq := &execproto.GetTransactionErrorMessageByIndexRequest{
		BlockId: suite.blockID[:],
		Index:   failedTxIndex,
	}
	exeEventResp := &execproto.GetTransactionErrorMessageResponse{
		TransactionId: failedTxId[:],
		ErrorMessage:  expectedErrorMsg,
	}
	suite.executionAPIClient.
		On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).
		Return(exeEventResp, nil).
		Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	suite.txResultErrorMessages.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByIndex(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxIndex,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedErrorMsg, errMsg)
}

// TestLookupTransactionErrorMessageByIndex_HappyPath verifies the lookup of a transaction error message
// by block ID and transaction index.
// It tests two cases:
// 1. Happy path where the error message is fetched from the EN if it is not found in the cache.
// 2. Happy path where the error message is served from the storage database if it exists.
func (suite *Suite) TestLookupTransactionErrorMessageByIndex_HappyPath() {
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()
	failedTxIndex := rand.Uint32()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()

	suite.Run("happy path from EN", func() {
		exeEventReq := &execproto.GetTransactionErrorMessageByIndexRequest{
			BlockId: suite.blockID[:],
			Index:   failedTxIndex,
		}
		exeEventResp := &execproto.GetTransactionErrorMessageResponse{
			TransactionId: failedTxId[:],
			ErrorMessage:  expectedErrorMsg,
		}
		suite.executionAPIClient.
			On("GetTransactionErrorMessageByIndex", mock.Anything, exeEventReq).
			Return(exeEventResp, nil).
			Once()

		suite.connectionFactory.
			On("GetExecutionAPIClient", mock.Anything).
			Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
			Once()

		suite.txResultErrorMessages.
			On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
			Return(nil, storage.ErrNotFound).
			Once()

		errMessageProvider := NewTxErrorMessageProvider(
			suite.log,
			suite.txResultErrorMessages,
			suite.txResultsIndex,
			suite.connectionFactory,
			suite.nodeCommunicator,
			suite.nodeProvider,
		)

		errMsg, err := errMessageProvider.ErrorMessageByIndex(context.Background(), suite.blockID, suite.block.Height, failedTxIndex)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
	})

	suite.Run("happy path from storage db", func() {
		errMessageProvider := NewTxErrorMessageProvider(
			suite.log,
			suite.txResultErrorMessages,
			suite.txResultsIndex,
			suite.connectionFactory,
			suite.nodeCommunicator,
			suite.nodeProvider,
		)

		suite.txResultErrorMessages.
			On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
			Return(&flow.TransactionResultErrorMessage{
				TransactionID: failedTxId,
				ErrorMessage:  expectedErrorMsg,
				Index:         failedTxIndex,
				ExecutorID:    unittest.IdentifierFixture(),
			}, nil).
			Once()

		errMsg, err := errMessageProvider.ErrorMessageByIndex(
			context.Background(),
			suite.blockID,
			suite.block.Height,
			failedTxIndex,
		)
		suite.Require().NoError(err)
		suite.Require().Equal(expectedErrorMsg, errMsg)
	})
}

func (suite *Suite) TestLookupByIndex_ExecutionNodeError_UnknownTx() {
	failedTxIndex := rand.Uint32()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	// Setup mock that the transaction and tx error message is not found in the storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(nil, storage.ErrNotFound).
		Once()
	suite.lightTxResults.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByIndex(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxIndex,
	)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupByIndex_ExecutionNodeError_TxResultNotFailed() {
	failedTxIndex := rand.Uint32()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	// Setup mock that the transaction error message is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(nil, storage.ErrNotFound).
		Once()

	// Setup mock that the transaction result exists and is not failed.
	suite.lightTxResults.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          false,
			ComputationUsed: 0,
		}, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByIndex(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxIndex,
	)
	suite.Require().NoError(err)
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupByIndex_ExecutionNodeError_TxResultFailed() {
	failedTxIndex := rand.Uint32()
	failedTx := unittest.TransactionFixture()
	failedTxId := failedTx.ID()

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessageByIndex", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// Setup mock that the transaction error message is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(nil, storage.ErrNotFound).
		Once()

	// Setup mock that the transaction result exists and is failed.
	suite.lightTxResults.
		On("ByBlockIDTransactionIndex", suite.blockID, failedTxIndex).
		Return(&flow.LightTransactionResult{
			TransactionID:   failedTxId,
			Failed:          true,
			ComputationUsed: 0,
		}, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessageByIndex(
		context.Background(),
		suite.blockID,
		suite.block.Height,
		failedTxIndex,
	)
	suite.Require().NoError(err)
	suite.Require().Equal(errMsg, DefaultFailedErrorMessage)
}

func (suite *Suite) TestLookupByBlockID_FromExecutionNode_HappyPath() {
	resultsByBlockID := make([]flow.LightTransactionResult, 0)
	for i := 0; i < 5; i++ {
		resultsByBlockID = append(resultsByBlockID, flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0, // create a mix of failed and non-failed transactions
			ComputationUsed: 0,
		})
	}

	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()

	// Mock the execution node API call to fetch the error messages.
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: suite.blockID[:],
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
	suite.executionAPIClient.
		On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(exeErrMessagesResp, nil).
		Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Once()

	suite.txResultErrorMessages.
		On("ByBlockID", suite.blockID).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMessages, err := errMessageProvider.ErrorMessagesByBlockID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
	)
	suite.Require().NoError(err)
	suite.Require().Len(errMessages, len(exeErrMessagesResp.Results))
	for _, expectedResult := range exeErrMessagesResp.Results {
		errMsg, ok := errMessages[convert.MessageToIdentifier(expectedResult.TransactionId)]
		suite.Require().True(ok)
		suite.Assert().Equal(expectedResult.ErrorMessage, errMsg)
	}
}

func (suite *Suite) TestLookupByBlockID_FromStorage_HappyPath() {
	resultsByBlockID := make([]flow.LightTransactionResult, 0)
	for i := 0; i < 5; i++ {
		resultsByBlockID = append(resultsByBlockID, flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0, // create a mix of failed and non-failed transactions
			ComputationUsed: 0,
		})
	}

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
	suite.txResultErrorMessages.
		On("ByBlockID", suite.blockID).
		Return(txErrorMessages, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMessages, err := errMessageProvider.ErrorMessagesByBlockID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
	)
	suite.Require().NoError(err)
	suite.Require().Len(errMessages, len(txErrorMessages))

	for _, expected := range txErrorMessages {
		errMsg, ok := errMessages[expected.TransactionID]
		suite.Require().True(ok)
		suite.Assert().Equal(expected.ErrorMessage, errMsg)
	}
}

func (suite *Suite) TestLookupByBlockID_ExecutionNodeError_UnknownBlock() {
	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	suite.executionAPIClient.
		On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	// Setup mock that the transaction and tx error messages is not found in the storage.
	suite.txResultErrorMessages.
		On("ByBlockID", suite.blockID).
		Return(nil, storage.ErrNotFound).
		Once()
	suite.lightTxResults.
		On("ByBlockID", suite.blockID).
		Return(nil, storage.ErrNotFound).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	// Perform the lookup and expect a "NotFound" error with an empty error message.
	errMsg, err := errMessageProvider.ErrorMessagesByBlockID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
	)
	suite.Require().Error(err)
	suite.Require().Equal(codes.NotFound, status.Code(err))
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupByBlockID_ExecutionNodeError_TxResultNotFailed() {
	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	// Setup mock that the transaction error message is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockID", suite.blockID).
		Return(nil, storage.ErrNotFound).
		Once()

	// Setup mock that the transaction results exists and is not failed.
	suite.lightTxResults.
		On("ByBlockID", suite.blockID).
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
		}, nil).
		Once()

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	errMsg, err := errMessageProvider.ErrorMessagesByBlockID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
	)
	suite.Require().NoError(err)
	suite.Require().Empty(errMsg)
}

func (suite *Suite) TestLookupTransactionErrorMessagesByBlockID_FailedToFetch() {
	suite.state.On("Final").Return(suite.snapshot, nil).Once()
	suite.snapshot.On("Identities", mock.Anything).Return(suite.fixedExecutionNodes, nil).Once()
	suite.reporter.On("LowestIndexedHeight").Return(suite.block.Height, nil).Once()
	suite.reporter.On("HighestIndexedHeight").Return(suite.block.Height+10, nil).Once()

	suite.connectionFactory.
		On("GetExecutionAPIClient", mock.Anything).
		Return(suite.executionAPIClient, &mocks.MockCloser{}, nil).
		Twice()

	// lookup should try each of the 2 ENs in fixedENIDs
	suite.executionAPIClient.
		On("GetTransactionErrorMessagesByBlockID", mock.Anything, mock.Anything).
		Return(nil, status.Error(codes.Unavailable, "")).
		Twice()

	// Setup mock that the transaction error messages is not found in storage.
	suite.txResultErrorMessages.
		On("ByBlockID", suite.blockID).
		Return(nil, storage.ErrNotFound).
		Once()

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

	suite.lightTxResults.
		On("ByBlockID", suite.blockID).
		Return(failedResultsByBlockID, nil).
		Once()

	expectedTxErrorMessages := make(map[flow.Identifier]string)
	for _, result := range failedResultsByBlockID {
		if result.Failed {
			expectedTxErrorMessages[result.TransactionID] = DefaultFailedErrorMessage
		}
	}

	errMessageProvider := NewTxErrorMessageProvider(
		suite.log,
		suite.txResultErrorMessages,
		suite.txResultsIndex,
		suite.connectionFactory,
		suite.nodeCommunicator,
		suite.nodeProvider,
	)

	// Perform the lookup and expect the failed error messages to be returned.
	errMsg, err := errMessageProvider.ErrorMessagesByBlockID(
		context.Background(),
		suite.blockID,
		suite.block.Height,
	)
	suite.Require().NoError(err)
	suite.Require().Len(errMsg, len(expectedTxErrorMessages))

	for txID, expectedMessage := range expectedTxErrorMessages {
		actualMessage, ok := errMsg[txID]
		suite.Require().True(ok)
		suite.Assert().Equal(expectedMessage, actualMessage)
	}
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
		Return(receipts, nil).
		Maybe()

	return receipts, ids
}
