package tx_error_messages

import (
	"context"
	"errors"
	"testing"
	"time"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/error_messages"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type RequesterSuite struct {
	suite.Suite

	log   zerolog.Logger
	proto struct {
		state    *protocol.FollowerState
		snapshot *protocol.Snapshot
		params   *protocol.Params
	}

	receipts    *storage.ExecutionReceipts
	enNodeIDs   flow.IdentityList
	execClient  *accessmock.ExecutionAPIClient
	connFactory *connectionmock.ConnectionFactory

	rootBlock      *flow.Block
	finalizedBlock *flow.Header

	txErrorMessages *storage.TransactionResultErrorMessages
	lightTxResults  *storage.LightTransactionResults
	reporter        *syncmock.IndexReporter
	indexReporter   *index.Reporter
	txResultsIndex  *index.TransactionResultsIndex
}

func TestRequester(t *testing.T) {
	suite.Run(t, new(RequesterSuite))
}

func (s *RequesterSuite) SetupTest() {
	s.log = unittest.Logger()
	s.proto.state = protocol.NewFollowerState(s.T())
	s.proto.snapshot = protocol.NewSnapshot(s.T())
	s.proto.params = protocol.NewParams(s.T())
	s.execClient = accessmock.NewExecutionAPIClient(s.T())
	s.connFactory = connectionmock.NewConnectionFactory(s.T())
	s.receipts = storage.NewExecutionReceipts(s.T())
	s.rootBlock = unittest.Block.Genesis(flow.Emulator)
	s.finalizedBlock = unittest.BlockWithParentFixture(s.rootBlock.ToHeader()).ToHeader()

	s.txErrorMessages = storage.NewTransactionResultErrorMessages(s.T())
	s.lightTxResults = storage.NewLightTransactionResults(s.T())
	s.reporter = syncmock.NewIndexReporter(s.T())
	s.indexReporter = index.NewReporter()
	err := s.indexReporter.Initialize(s.reporter)
	s.Require().NoError(err)
	s.txResultsIndex = index.NewTransactionResultsIndex(s.indexReporter, s.lightTxResults)

	s.proto.params.On("FinalizedRoot").Return(s.rootBlock.ToHeader(), nil)
	s.proto.state.On("Params").Return(s.proto.params)

	s.proto.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.finalizedBlock
		},
		nil,
	).Maybe()
	s.proto.state.On("Final").Return(s.proto.snapshot, nil)

	s.enNodeIDs = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
}

func (s *RequesterSuite) TestRequest_HappyPath() {
	execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.proto.state,
		s.receipts,
		flow.IdentifierList{},
		s.enNodeIDs.NodeIDs(),
	)

	errorMessageProvider := error_messages.NewTxErrorMessageProvider(
		s.log,
		s.txErrorMessages,
		s.txResultsIndex,
		s.connFactory,
		node_communicator.NewNodeCommunicator(false),
		execNodeIdentitiesProvider,
	)

	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	blockId := block.ID()
	executionResult := &flow.ExecutionResult{
		BlockID: blockId,
		Chunks:  unittest.ChunkListFixture(1, blockId, unittest.StateCommitmentFixture()),
	}
	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	// Mock the protocol snapshot to return fixed execution node IDs.
	setupReceiptsForBlockWithResult(s.receipts, executionResult, s.enNodeIDs.NodeIDs()...)
	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)

	// Create mock transaction results with a mix of failed and non-failed transactions.
	resultsByBlockID := mockTransactionResultsByBlock(5)
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}
	s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).
		Once()

	expectedErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])
	config := &RequesterConfig{
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 5 * time.Second,
	}
	requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
	actualErrorMessages, err := requester.Request(context.Background())
	require.NoError(s.T(), err)
	require.ElementsMatch(s.T(), expectedErrorMessages, actualErrorMessages)
}

func (s *RequesterSuite) TestRequest_ErrorCases() {
	execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.proto.state,
		s.receipts,
		flow.IdentifierList{},
		s.enNodeIDs.NodeIDs(),
	)

	errorMessageProvider := error_messages.NewTxErrorMessageProvider(
		s.log,
		s.txErrorMessages,
		s.txResultsIndex,
		s.connFactory,
		node_communicator.NewNodeCommunicator(false),
		execNodeIdentitiesProvider,
	)

	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	blockId := block.ID()
	executionResult := &flow.ExecutionResult{
		BlockID: blockId,
		Chunks:  unittest.ChunkListFixture(1, blockId, unittest.StateCommitmentFixture()),
	}
	config := &RequesterConfig{
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 5 * time.Second,
	}

	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	// Mock the protocol snapshot to return fixed execution node IDs.
	setupReceiptsForBlockWithResult(s.receipts, executionResult, s.enNodeIDs.NodeIDs()...)
	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)

	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}

	s.T().Run("Non-retryable error", func(t *testing.T) {
		expectedError := errors.New("non-retryable error")
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, expectedError).
			Once()

		requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
		actualErrorMessages, err := requester.Request(context.Background())
		require.ErrorIs(s.T(), err, expectedError)
		require.Nil(s.T(), actualErrorMessages)
	})

	s.T().Run("Non-retryable grpc DeadlineExceeded error", func(t *testing.T) {
		expectedError := status.Error(codes.DeadlineExceeded, "deadline exceeded")
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, expectedError).
			Once()

		requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
		actualErrorMessages, err := requester.Request(context.Background())
		require.ErrorIs(s.T(), err, expectedError)
		require.Nil(s.T(), actualErrorMessages)
	})

	s.T().Run("Non-retryable grpc Canceled error", func(t *testing.T) {
		expectedError := status.Error(codes.Canceled, "context canceled")
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, expectedError).
			Once()

		requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
		actualErrorMessages, err := requester.Request(context.Background())
		require.ErrorIs(s.T(), err, expectedError)
		require.Nil(s.T(), actualErrorMessages)
	})

	s.T().Run("Retryable ErrNoENsFoundForExecutionResult error", func(t *testing.T) {
		// first time return retryable error
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, commonrpc.ErrNoENsFoundForExecutionResult).
			Once()

		// second time return error messages
		resultsByBlockID := mockTransactionResultsByBlock(5)
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).
			Once()

		expectedErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])
		requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
		actualErrorMessages, err := requester.Request(context.Background())
		require.NoError(s.T(), err)
		require.ElementsMatch(s.T(), expectedErrorMessages, actualErrorMessages)
	})

	s.T().Run("Retryable valid grpc error", func(t *testing.T) {
		// first time return retryable error
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, status.Error(codes.NotFound, "not found")).
			Once()

		// second time return error messages
		resultsByBlockID := mockTransactionResultsByBlock(5)
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).
			Once()

		expectedErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])
		requester := NewRequester(s.log, config, errorMessageProvider, execNodeIdentitiesProvider, executionResult)
		actualErrorMessages, err := requester.Request(context.Background())
		require.NoError(s.T(), err)
		require.ElementsMatch(s.T(), expectedErrorMessages, actualErrorMessages)
	})
}
