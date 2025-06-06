package tx_error_messages

import (
	"context"
	"testing"
	"time"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type TxErrorMessagesRequesterSuite struct {
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

	rootBlock      flow.Block
	finalizedBlock *flow.Header
}

func TestTxErrorMessagesRequester(t *testing.T) {
	suite.Run(t, new(TxErrorMessagesRequesterSuite))
}

func (s *TxErrorMessagesRequesterSuite) SetupTest() {
	s.log = unittest.Logger()
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

	s.proto.snapshot.On("Head").Return(
		func() *flow.Header {
			return s.finalizedBlock
		},
		nil,
	).Maybe()
	s.proto.state.On("Final").Return(s.proto.snapshot, nil)

	s.enNodeIDs = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
}

func (s *TxErrorMessagesRequesterSuite) TestTxErrorMessageRequester_RequestErrorMessages() {
	execNodeIdentitiesProvider := commonrpc.NewExecutionNodeIdentitiesProvider(
		s.log,
		s.proto.state,
		s.receipts,
		flow.IdentifierList{},
		s.enNodeIDs.NodeIDs(),
	)

	back, err := backend.New(backend.Params{
		State:                      s.proto.state,
		ExecutionReceipts:          s.receipts,
		ConnFactory:                s.connFactory,
		MaxHeightRange:             backend.DefaultMaxHeightRange,
		Log:                        s.log,
		SnapshotHistoryLimit:       backend.DefaultSnapshotHistoryLimit,
		Communicator:               backend.NewNodeCommunicator(false),
		ScriptExecutionMode:        backend.IndexQueryModeExecutionNodesOnly,
		TxResultQueryMode:          backend.IndexQueryModeExecutionNodesOnly,
		ChainID:                    flow.Testnet,
		ExecNodeIdentitiesProvider: execNodeIdentitiesProvider,
	})
	require.NoError(s.T(), err)

	config := &TransactionErrorMessagesRequesterConfig{
		RetryDelay:    1 * time.Second,
		MaxRetryDelay: 5 * time.Second,
	}

	block := unittest.BlockWithParentFixture(s.finalizedBlock)
	blockId := block.ID()
	executionResult := &flow.ExecutionResult{
		BlockID: blockId,
		Chunks:  unittest.ChunkListFixture(1, blockId, unittest.StateCommitmentFixture()),
	}
	s.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(s.execClient, &mockCloser{}, nil)

	// Mock the protocol snapshot to return fixed execution node IDs.
	setupReceiptsForBlockWithResult(s.receipts, block, s.enNodeIDs.NodeIDs()[0], *executionResult)
	s.proto.snapshot.On("Identities", mock.Anything).Return(s.enNodeIDs, nil)

	// Create mock transaction results with a mix of failed and non-failed transactions.
	resultsByBlockID := mockTransactionResultsByBlock(5)
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockId[:],
	}
	s.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(createTransactionErrorMessagesResponse(resultsByBlockID), nil).
		Once()

	// Prepare the expected transaction error messages that should be stored.
	expectedErrorMessages := createExpectedTxErrorMessages(resultsByBlockID, s.enNodeIDs.NodeIDs()[0])
	requester := NewTransactionErrorMessagesRequester(s.log, config, back, execNodeIdentitiesProvider, executionResult)
	actualErrorMessages, err := requester.RequestErrorMessages(context.Background())
	require.NoError(s.T(), err)
	require.ElementsMatch(s.T(), expectedErrorMessages, actualErrorMessages)
}
