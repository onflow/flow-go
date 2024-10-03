package storage

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

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/engine/access/index"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	syncmock "github.com/onflow/flow-go/module/state_synchronization/mock"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const expectedErrorMsg = "expected test error"

type BackfillTxErrorMessagesSuite struct {
	suite.Suite

	command commands.AdminCommand

	log      zerolog.Logger
	state    *protocolmock.State
	snapshot *protocolmock.Snapshot
	params   *protocolmock.Params

	txErrorMessages    *storagemock.TransactionResultErrorMessages
	transactionResults *storagemock.LightTransactionResults
	receipts           *storagemock.ExecutionReceipts
	headers            *storagemock.Headers

	execClient *accessmock.ExecutionAPIClient

	connFactory *connectionmock.ConnectionFactory
	reporter    *syncmock.IndexReporter
	allENIDs    flow.IdentityList

	backend *backend.Backend

	blockHeadersMap map[uint64]*flow.Header

	nodeRootBlock flow.Block
	blockCount    int
}

func TestBackfillTxErrorMessages(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(BackfillTxErrorMessagesSuite))
}

type mockCloser struct{}

func (mc *mockCloser) Close() error { return nil }

func (suite *BackfillTxErrorMessagesSuite) SetupTest() {
	suite.log = zerolog.New(os.Stderr)

	suite.state = new(protocolmock.State)
	suite.headers = new(storagemock.Headers)
	suite.receipts = new(storagemock.ExecutionReceipts)
	suite.transactionResults = storagemock.NewLightTransactionResults(suite.T())
	suite.txErrorMessages = new(storagemock.TransactionResultErrorMessages)
	suite.execClient = new(accessmock.ExecutionAPIClient)

	suite.blockCount = 5
	suite.blockHeadersMap = make(map[uint64]*flow.Header, suite.blockCount)
	suite.nodeRootBlock = unittest.BlockFixture()
	suite.nodeRootBlock.Header.Height = 0
	suite.blockHeadersMap[suite.nodeRootBlock.Header.Height] = suite.nodeRootBlock.Header

	parent := suite.nodeRootBlock.Header

	for i := 1; i <= suite.blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.Header
		suite.blockHeadersMap[block.Header.Height] = block.Header
	}

	suite.params = protocolmock.NewParams(suite.T())
	suite.params.On("SealedRoot").Return(suite.nodeRootBlock.Header, nil)
	suite.state.On("Params").Return(suite.params, nil).Maybe()

	suite.snapshot = createSnapshot(parent)
	suite.state.On("Sealed").Return(suite.snapshot)
	suite.state.On("Final").Return(suite.snapshot)

	suite.state.On("AtHeight", mock.Anything).Return(
		func(height uint64) protocol.Snapshot {
			if int(height) < len(suite.blockHeadersMap) {
				header := suite.blockHeadersMap[height]
				return createSnapshot(header)
			}
			return invalid.NewSnapshot(fmt.Errorf("invalid height: %v", height))
		},
	)

	// Mock the protocol snapshot to return fixed execution node IDs.
	suite.allENIDs = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(flow.IdentityFilter[flow.Identity]) (flow.IdentityList, error) {
			return suite.allENIDs, nil
		}, nil)

	// create a mock connection factory
	suite.connFactory = connectionmock.NewConnectionFactory(suite.T())

	// Create a mock index reporter
	suite.reporter = syncmock.NewIndexReporter(suite.T())

	txResultsIndex := index.NewTransactionResultsIndex(index.NewReporter(), suite.transactionResults)
	err := txResultsIndex.Initialize(suite.reporter)
	suite.Require().NoError(err)

	suite.backend, err = backend.New(backend.Params{
		State:                suite.state,
		ExecutionReceipts:    suite.receipts,
		ConnFactory:          suite.connFactory,
		MaxHeightRange:       backend.DefaultMaxHeightRange,
		Log:                  suite.log,
		SnapshotHistoryLimit: backend.DefaultSnapshotHistoryLimit,
		Communicator:         backend.NewNodeCommunicator(false),
		ScriptExecutionMode:  backend.IndexQueryModeExecutionNodesOnly,
		TxResultQueryMode:    backend.IndexQueryModeExecutionNodesOnly,
		ChainID:              flow.Testnet,
		ExecNodeIdentitiesProvider: commonrpc.NewExecutionNodeIdentitiesProvider(
			suite.log,
			suite.state,
			suite.receipts,
			nil,
			nil,
		),
	})
	require.NoError(suite.T(), err)

	suite.command = NewBackfillTxErrorMessagesCommand(
		suite.state,
		txResultsIndex,
		suite.txErrorMessages,
		suite.backend,
	)
}

func (suite *BackfillTxErrorMessagesSuite) TestValidateInvalidFormat() {
	suite.Run("invalid start-height field", func() {
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height": "123",
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqErrorf(
			"invalid 'start-height' field: %w",
			fmt.Errorf("invalid value for \"n\": %v", 0)))
	})

	suite.Run("invalid end-height field", func() {
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"end-height": "123",
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqErrorf(
			"invalid 'end-height' field: %w",
			fmt.Errorf("invalid value for \"n\": %v", 0)))
	})

	suite.Run("invalid combination of start-height and end-height fields", func() {
		startHeight := 10
		endHeight := 1
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height": float64(startHeight), // raw json parses to float64
				"end-height":   float64(endHeight),   // raw json parses to float64
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqErrorf(
			"start-height %v should not be smaller than end-height %v", startHeight, endHeight))
	})

	// invalid execution-node-ids param
	suite.Run("invalid execution-node-ids field", func() {
		// invalid type
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"execution-node-ids": []int{1, 2, 3},
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqParameterError(
			"execution-node-ids", "must be a list of string", []int{1, 2, 3}))

		// invalid type
		err = suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"execution-node-ids": "123",
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqParameterError(
			"execution-node-ids", "must be a list of string", "123"))

		// invalid execution node id
		invalidENID := unittest.IdentifierFixture()
		err = suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height":       float64(1),  // raw json parses to float64
				"end-height":         float64(10), // raw json parses to float64
				"execution-node-ids": []string{invalidENID.String()},
			},
		})
		suite.Error(err)
		suite.Equal(err, admin.NewInvalidAdminReqParameterError(
			"execution-node-ids", "could not found execution nodes by provided ids", []string{invalidENID.String()}))
	})
}

func (suite *BackfillTxErrorMessagesSuite) TestValidateValidFormat() {
	// start-height and end-height are not provided, the root block and the latest sealed block
	// will be used as the start and end heights respectively.
	// execution-node-ids is not provided, any valid execution node will be used.
	suite.Run("happy case, all default parameters", func() {
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{},
		})
		suite.NoError(err)
	})

	// all parameters are provided
	// start-height is less than root block, the root  block
	// will be used as the start-height.
	suite.Run("happy case, start-height is less than root block", func() {
		suite.params.On("SealedRoot").Return(suite.blockHeadersMap[1].Height, nil)
		suite.state.On("Params").Return(suite.params, nil).Maybe()
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height":       float64(2), // raw json parses to float64
				"end-height":         float64(5), // raw json parses to float64
				"execution-node-ids": []string{suite.allENIDs[0].ID().String()},
			},
		})
		suite.NoError(err)
	})

	// all parameters are provided
	// end-height is bigger than latest sealed block, the latest sealed block
	// will be used as the end-height.
	suite.Run("happy case, end-height is bigger than latest sealed block", func() {
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height":       float64(1),   // raw json parses to float64
				"end-height":         float64(100), // raw json parses to float64
				"execution-node-ids": []string{suite.allENIDs[0].ID().String()},
			},
		})
		suite.NoError(err)
	})

	// all parameters are provided
	suite.Run("happy case, all parameters are provided", func() {
		err := suite.command.Validator(&admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height":       float64(1), // raw json parses to float64
				"end-height":         float64(3), // raw json parses to float64
				"execution-node-ids": []string{suite.allENIDs[0].ID().String()},
			},
		})
		suite.NoError(err)
	})
}

func (suite *BackfillTxErrorMessagesSuite) TestHandleBackfillTxErrorMessages() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// default parameters
	req := &admin.CommandRequest{
		Data: map[string]interface{}{},
	}
	suite.Require().NoError(suite.command.Validator(req))

	suite.Run("happy case, all default parameters, tx error messages do not exist in db", func() {
		suite.reporter.On("LowestIndexedHeight").Return(suite.nodeRootBlock.Header.Height, nil)
		suite.reporter.On("HighestIndexedHeight").Return(suite.blockHeadersMap[uint64(suite.blockCount)].Height, nil)

		// Create a mock execution client to simulate communication with execution nodes.
		suite.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

		for i := suite.nodeRootBlock.Header.Height; i <= suite.blockHeadersMap[uint64(suite.blockCount)].Height; i++ {
			blockId := suite.blockHeadersMap[i].ID()

			// Setup mock storing the transaction error message after retrieving the failed result.
			suite.txErrorMessages.On("Exists", blockId).Return(false, nil).Once()

			results := suite.generateResultsForBlock()

			// Setup mock that the transaction result exists and is failed.
			suite.transactionResults.On("ByBlockID", blockId).
				Return(results, nil).Once()

			// Mock the execution node API calls to fetch the error messages.
			suite.mockTransactionErrorMessagesResponseByBlockID(blockId, results)

			// Setup mock storing the transaction error message after retrieving the failed result.
			suite.mockStoreTxErrorMessages(blockId, results, suite.allENIDs[0].ID())
		}

		_, err := suite.command.Handler(ctx, req)
		suite.Require().NoError(err)
		suite.assertAllExpectations()
	})

	suite.Run("happy case, all default parameters, tx error messages exist in db", func() {
		for i := suite.nodeRootBlock.Header.Height; i <= suite.blockHeadersMap[uint64(suite.blockCount)].Height; i++ {
			blockId := suite.blockHeadersMap[i].ID()

			// Setup mock storing the transaction error message after retrieving the failed result.
			suite.txErrorMessages.On("Exists", blockId).Return(true, nil).Once()
		}

		_, err := suite.command.Handler(ctx, req)
		suite.Require().NoError(err)
		suite.assertAllExpectations()
	})

	suite.Run("happy case, all custom parameters, tx error messages do not exist in db", func() {
		// custom parameters
		startHeight := 1
		endHeight := 4

		suite.allENIDs = unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleExecution))

		executorID := suite.allENIDs[1].ID()
		req = &admin.CommandRequest{
			Data: map[string]interface{}{
				"start-height":       float64(startHeight), // raw json parses to float64
				"end-height":         float64(endHeight),   // raw json parses to float64
				"execution-node-ids": []string{executorID.String()},
			},
		}
		suite.Require().NoError(suite.command.Validator(req))

		suite.reporter.On("LowestIndexedHeight").Return(suite.nodeRootBlock.Header.Height, nil)
		suite.reporter.On("HighestIndexedHeight").Return(suite.blockHeadersMap[uint64(endHeight)].Height, nil)

		// Create a mock execution client to simulate communication with execution nodes.
		suite.connFactory.On("GetExecutionAPIClient", mock.Anything).Return(suite.execClient, &mockCloser{}, nil)

		for i := startHeight; i <= endHeight; i++ {
			blockId := suite.blockHeadersMap[uint64(i)].ID()

			// Setup mock storing the transaction error message after retrieving the failed result.
			suite.txErrorMessages.On("Exists", blockId).Return(false, nil).Once()

			results := suite.generateResultsForBlock()

			// Setup mock that the transaction result exists and is failed.
			suite.transactionResults.On("ByBlockID", blockId).
				Return(results, nil).Once()

			// Mock the execution node API calls to fetch the error messages.
			suite.mockTransactionErrorMessagesResponseByBlockID(blockId, results)

			// Setup mock storing the transaction error message after retrieving the failed result.
			suite.mockStoreTxErrorMessages(blockId, results, executorID)
		}

		_, err := suite.command.Handler(ctx, req)
		suite.Require().NoError(err)
		suite.assertAllExpectations()
	})
}

func (suite *BackfillTxErrorMessagesSuite) TestHandleBackfillTxErrorMessagesErrors() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// default parameters
	req := &admin.CommandRequest{
		Data: map[string]interface{}{},
	}
	suite.Require().NoError(suite.command.Validator(req))

	suite.Run("failed, not indexed tx results", func() {
		suite.reporter.On("LowestIndexedHeight").Return(suite.nodeRootBlock.Header.Height, nil)
		suite.reporter.On("HighestIndexedHeight").Return(suite.nodeRootBlock.Header.Height, nil)

		expectedErr := fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)

		blockID := suite.nodeRootBlock.Header.ID()
		suite.txErrorMessages.On("Exists", blockID).Return(false, nil).Once()

		// Setup mock that the transaction result exists and is failed.
		suite.transactionResults.On("ByBlockID", blockID).
			Return(nil, expectedErr).Once()

		_, err := suite.command.Handler(ctx, req)
		suite.Require().Error(err)
		suite.Equal(err, fmt.Errorf("failed to get result by block ID: %w", expectedErr))
		suite.assertAllExpectations()
	})
}

func (suite *BackfillTxErrorMessagesSuite) generateResultsForBlock() []flow.LightTransactionResult {
	results := make([]flow.LightTransactionResult, 0)

	for i := 0; i < 5; i++ {
		results = append(results, flow.LightTransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			Failed:          i%2 == 0, // create a mix of failed and non-failed transactions
			ComputationUsed: 0,
		})
	}

	return results
}

func (suite *BackfillTxErrorMessagesSuite) mockTransactionErrorMessagesResponseByBlockID(
	blockID flow.Identifier,
	results []flow.LightTransactionResult,
) {
	exeEventReq := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: blockID[:],
	}

	exeErrMessagesResp := &execproto.GetTransactionErrorMessagesResponse{}
	for i, result := range results {
		r := result
		if r.Failed {
			errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, r.TransactionID)
			exeErrMessagesResp.Results = append(exeErrMessagesResp.Results, &execproto.GetTransactionErrorMessagesResponse_Result{
				TransactionId: r.TransactionID[:],
				ErrorMessage:  errMsg,
				Index:         uint32(i),
			})
		}
	}

	suite.execClient.On("GetTransactionErrorMessagesByBlockID", mock.Anything, exeEventReq).
		Return(exeErrMessagesResp, nil).
		Once()
}

func (suite *BackfillTxErrorMessagesSuite) mockStoreTxErrorMessages(
	blockID flow.Identifier,
	results []flow.LightTransactionResult,
	executorID flow.Identifier,
) {
	var txErrorMessages []flow.TransactionResultErrorMessage

	for i, result := range results {
		r := result
		if r.Failed {
			errMsg := fmt.Sprintf("%s.%s", expectedErrorMsg, r.TransactionID)

			txErrorMessages = append(txErrorMessages,
				flow.TransactionResultErrorMessage{
					TransactionID: result.TransactionID,
					ErrorMessage:  errMsg,
					Index:         uint32(i),
					ExecutorID:    executorID,
				})
		}
	}

	suite.txErrorMessages.On("Store", blockID, txErrorMessages).Return(nil).Once()
}

func (suite *BackfillTxErrorMessagesSuite) assertAllExpectations() {
	suite.snapshot.AssertExpectations(suite.T())
	suite.state.AssertExpectations(suite.T())
	suite.headers.AssertExpectations(suite.T())
	suite.execClient.AssertExpectations(suite.T())
	suite.transactionResults.AssertExpectations(suite.T())
	suite.reporter.AssertExpectations(suite.T())
	suite.txErrorMessages.AssertExpectations(suite.T())
}
