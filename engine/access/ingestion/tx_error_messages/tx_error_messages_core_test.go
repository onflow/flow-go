package tx_error_messages

import (
	"context"
	"fmt"
	"os"
	"testing"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const expectedErrorMsg = "expected test error"

type TxErrorMessagesCoreSuite struct {
	suite.Suite

	log   zerolog.Logger
	proto struct {
		state    *protocol.FollowerState
		snapshot *protocol.Snapshot
		params   *protocol.Params
	}

	receipts        *storage.ExecutionReceipts
	txErrorMessages *storage.TransactionResultErrorMessages

	enNodeIDs   flow.IdentityList
	execClient  *accessmock.ExecutionAPIClient
	connFactory *connectionmock.ConnectionFactory

	blockMap       map[uint64]*flow.Block
	rootBlock      flow.Block
	finalizedBlock *flow.Header

	ctx    context.Context
	cancel context.CancelFunc
}

func TestTxErrorMessagesCore(t *testing.T) {
	suite.Run(t, new(TxErrorMessagesCoreSuite))
}

// TearDownTest stops the engine and cleans up the db
func (s *TxErrorMessagesCoreSuite) TearDownTest() {
	s.cancel()
}

type mockCloser struct{}

func (mc *mockCloser) Close() error { return nil }

func (s *TxErrorMessagesCoreSuite) SetupTest() {
	s.log = zerolog.New(os.Stderr)
	s.ctx, s.cancel = context.WithCancel(context.Background())
	// mock out protocol state
	s.proto.state = protocol.NewFollowerState(s.T())
	s.proto.snapshot = protocol.NewSnapshot(s.T())
	s.proto.params = protocol.NewParams(s.T())
	s.execClient = accessmock.NewExecutionAPIClient(s.T())
	s.connFactory = connectionmock.NewConnectionFactory(s.T())
	s.receipts = storage.NewExecutionReceipts(s.T())
	s.txErrorMessages = storage.NewTransactionResultErrorMessages(s.T())

	s.rootBlock = unittest.BlockFixture()
	s.rootBlock.Header.Height = 0
	s.finalizedBlock = unittest.BlockWithParentFixture(s.rootBlock.Header).Header

	s.proto.state.On("Params").Return(s.proto.params)
	s.proto.params.On("SporkID").Return(unittest.IdentifierFixture(), nil)
	s.proto.params.On("ProtocolVersion").Return(uint(1), nil)
	s.proto.params.On("SporkRootBlockHeight").Return(uint64(0), nil)
	block := unittest.BlockFixture()
	s.proto.params.On("SealedRoot").Return(block.Header, nil)

	// Mock the finalized root block header with height 0.
	s.proto.params.On("FinalizedRoot").Return(s.rootBlock.Header, nil)

	s.proto.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.finalizedBlock
		},
		nil,
	).Maybe()
	s.proto.state.On("Final").Return(s.proto.snapshot, nil)

	// Create identities for 1 execution nodes.
	s.enNodeIDs = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
}

// TestHandleTransactionResultErrorMessages checks that transaction result error messages
// are properly fetched from the execution nodes, processed, and stored in the protocol database.
func (s *TxErrorMessagesCoreSuite) TestHandleTransactionResultErrorMessages() {
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(s.T(), s.ctx)

	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	blockId := block.ID()

	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	// Mock the protocol snapshot to return fixed execution node IDs.
	setupReceiptsForBlock(s.receipts, block, s.enNodeIDs.NodeIDs()[0])
	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)
	s.proto.state.On("AtBlockID", blockId).Return(s.proto.snapshot).Once()

	// Create mock transaction results with a mix of failed and non-failed transactions.
	resultsByBlockID := mockTransactionResultsByBlock(5)

	// Prepare a request to fetch transaction error messages by block ID from execution nodes.
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}

	// Mock the txErrorMessages storage to confirm that error messages do not exist yet.
	s.txErrorMessages.On("Exists", blockId).
		Return(false, nil).Once()

	// Prepare the expected transaction error messages that should be stored.
	expectedStoreTxErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])

	grpcResponse := createTransactionErrorMessagesResponse(resultsByBlockID)
	s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(grpcResponse, nil).
		Once()

	// Mock the storage of the fetched error messages into the protocol database.
	s.txErrorMessages.On("Store", blockId, expectedStoreTxErrorMessages).
		Return(nil).Once()

	core := s.initCore()
	err := core.HandleTransactionResultErrorMessages(irrecoverableCtx, blockId)
	require.NoError(s.T(), err)

	// Verify that the mock expectations for storing the error messages were met.
	s.txErrorMessages.AssertExpectations(s.T())

	// Now simulate the second try when the error messages already exist in storage.
	// Mock the txErrorMessages storage to confirm that error messages exist.
	s.txErrorMessages.On("Exists", blockId).
		Return(true, nil).Once()
	err = core.HandleTransactionResultErrorMessages(irrecoverableCtx, blockId)
	require.NoError(s.T(), err)

	// Verify that the mock expectations for storing the error messages were not met.
	s.txErrorMessages.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}

// TestHandleTransactionResultErrorMessages_ErrorCases tests the error handling of
// the HandleTransactionResultErrorMessages function in the following cases:
//
// 1. Execution node fetch error: When fetching transaction error messages from the execution node fails,
// the function should return an appropriate error and no further actions should be taken.
// 2. Storage store error after fetching results: When fetching transaction error messages succeeds,
// but storing them in the storage fails, the function should return an error and no further actions should be taken.
func (s *TxErrorMessagesCoreSuite) TestHandleTransactionResultErrorMessages_ErrorCases() {
	irrecoverableCtx := irrecoverable.NewMockSignalerContext(s.T(), s.ctx)

	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	blockId := block.ID()

	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	// Mock the protocol snapshot to return fixed execution node IDs.
	setupReceiptsForBlock(s.receipts, block, s.enNodeIDs.NodeIDs()[0])
	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)
	s.proto.state.On("AtBlockID", blockId).Return(s.proto.snapshot)

	s.Run("Execution node fetch error", func() {
		// Mock the txErrorMessages storage to confirm that error messages do not exist yet.
		s.txErrorMessages.On("Exists", blockId).Return(false, nil).Once()

		// Simulate an error when fetching transaction error messages from the execution node.
		exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
			BlockId: blockId[:],
		}
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(nil, fmt.Errorf("execution node fetch error")).Once()

		core := s.initCore()
		err := core.HandleTransactionResultErrorMessages(irrecoverableCtx, blockId)

		// Assert that the function returns an error due to the client fetch error.
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "execution node fetch error")

		// Ensure that no further steps are taken after the client fetch error.
		s.txErrorMessages.AssertNotCalled(s.T(), "Store", mock.Anything, mock.Anything)
	})

	s.Run("Storage error after fetching results", func() {
		// Simulate successful fetching of transaction error messages but error in storing them.

		// Mock the txErrorMessages storage to confirm that error messages do not exist yet.
		s.txErrorMessages.On("Exists", blockId).Return(false, nil).Once()

		// Create mock transaction results with a mix of failed and non-failed transactions.
		resultsByBlockID := mockTransactionResultsByBlock(5)

		// Prepare a request to fetch transaction error messages by block ID from execution nodes.
		exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
			BlockId: blockId[:],
		}
		s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
			Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).Once()

		// Simulate an error when attempting to store the fetched transaction error messages in storage.
		expectedStoreTxErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])
		s.txErrorMessages.On("Store", blockId, expectedStoreTxErrorMessages).
			Return(fmt.Errorf("storage error")).Once()

		core := s.initCore()
		err := core.HandleTransactionResultErrorMessages(irrecoverableCtx, blockId)

		// Assert that the function returns an error due to the store error.
		require.Error(s.T(), err)
		require.Contains(s.T(), err.Error(), "storage error")

		// Ensure that storage existence check and transaction fetch were called before the store error.
		s.txErrorMessages.AssertCalled(s.T(), "Exists", blockId)
		s.execClient.AssertCalled(s.T(), "GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq)
	})
}

// initCore create new instance of transaction error messages core.
func (s *TxErrorMessagesCoreSuite) initCore() *TxErrorMessagesCore {
	// Initialize the backend
	backend, err := backend.New(backend.Params{
		State:                 s.proto.state,
		ExecutionReceipts:     s.receipts,
		ConnFactory:           s.connFactory,
		MaxHeightRange:        backend.DefaultMaxHeightRange,
		FixedExecutionNodeIDs: s.enNodeIDs.NodeIDs().Strings(),
		Log:                   s.log,
		SnapshotHistoryLimit:  backend.DefaultSnapshotHistoryLimit,
		Communicator:          backend.NewNodeCommunicator(false),
		ScriptExecutionMode:   backend.IndexQueryModeExecutionNodesOnly,
		TxResultQueryMode:     backend.IndexQueryModeExecutionNodesOnly,
		ChainID:               flow.Testnet,
	})
	require.NoError(s.T(), err)

	core := NewTxErrorMessagesCore(
		s.log,
		s.proto.state,
		backend,
		s.receipts,
		s.txErrorMessages,
		s.enNodeIDs.NodeIDs(),
		nil,
	)
	return core
}

// createExpectedTxErrorMessages creates a list of expected transaction error messages based on transaction results
func createExpectedTxErrorMessages(resultsByBlockID []flow.LightTransactionResult, executionNode flow.Identifier) []flow.TransactionResultErrorMessage {
	// Prepare the expected transaction error messages that should be stored.
	var expectedStoreTxErrorMessages []flow.TransactionResultErrorMessage

	for i, result := range resultsByBlockID {
		if result.Failed {
			errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, result.TransactionID)

			expectedStoreTxErrorMessages = append(expectedStoreTxErrorMessages,
				flow.TransactionResultErrorMessage{
					TransactionID: result.TransactionID,
					ErrorMessage:  errMsg,
					Index:         uint32(i),
					ExecutorID:    executionNode,
				})
		}
	}

	return expectedStoreTxErrorMessages
}

// mockTransactionResultsByBlock create mock transaction results with a mix of failed and non-failed transactions.
func mockTransactionResultsByBlock(count int) []flow.LightTransactionResult {
	// Create mock transaction results with a mix of failed and non-failed transactions.
	resultsByBlockID := make([]flow.LightTransactionResult, 0)
	for i := 0; i < count; i++ {
		resultsByBlockID = append(resultsByBlockID, flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(), //TODO(illia): is this correct ?
			Failed:          i%2 == 0,                     // create a mix of failed and non-failed transactions
			ComputationUsed: 0,
		})
	}

	return resultsByBlockID
}

// setupReceiptsForBlock sets up mock execution receipts for a block and returns the receipts along
// with the identities of the execution nodes that processed them.
func setupReceiptsForBlock(receipts *storage.ExecutionReceipts, block *flow.Block, eNodeID flow.Identifier) {
	receipt1 := unittest.ReceiptForBlockFixture(block)
	receipt1.ExecutorID = eNodeID
	receipt2 := unittest.ReceiptForBlockFixture(block)
	receipt2.ExecutorID = eNodeID
	receipt1.ExecutionResult = receipt2.ExecutionResult

	receiptsList := flow.ExecutionReceiptList{receipt1, receipt2}

	receipts.
		On("ByBlockID", block.ID()).
		Return(func(flow.Identifier) flow.ExecutionReceiptList {
			return receiptsList
		}, nil)
}

// createTransactionErrorMessagesResponse create TransactionErrorMessagesResponse from execution node based on results.
func createTransactionErrorMessagesResponse(resultsByBlockID []flow.LightTransactionResult) *execproto.GetTransactionErrorMessagesResponse {
	exeErrMessagesResp := &execproto.GetTransactionErrorMessagesResponse{}

	for i := range resultsByBlockID {
		result := resultsByBlockID[i]
		if result.Failed {
			errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, result.TransactionID)
			exeErrMessagesResp.Results = append(exeErrMessagesResp.Results, &execproto.GetTransactionErrorMessagesResponse_Result{
				TransactionId: result.TransactionID[:],
				ErrorMessage:  errMsg,
				Index:         uint32(i),
			})
		}
	}

	return exeErrMessagesResp
}
