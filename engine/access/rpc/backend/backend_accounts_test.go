package backend

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	access "github.com/onflow/flow-go/engine/access/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
)

type BackendAccountsSuite struct {
	suite.Suite

	log        zerolog.Logger
	state      *protocol.State
	snapshot   *protocol.Snapshot
	params     *protocol.Params
	rootHeader *flow.Header

	headers           *storagemock.Headers
	receipts          *storagemock.ExecutionReceipts
	connectionFactory *connectionmock.ConnectionFactory
	chainID           flow.ChainID

	executionNodes flow.IdentityList
	execClient     *access.ExecutionAPIClient

	block          *flow.Block
	account        *flow.Account
	failingAddress flow.Address
}

func TestBackendAccountsSuite(t *testing.T) {
	suite.Run(t, new(BackendAccountsSuite))
}

func (s *BackendAccountsSuite) SetupTest() {
	s.log = unittest.Logger()
	s.state = protocol.NewState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.rootHeader = unittest.BlockHeaderFixture()
	s.params = protocol.NewParams(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.chainID = flow.Testnet

	s.execClient = access.NewExecutionAPIClient(s.T())
	s.executionNodes = unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))

	block := unittest.BlockFixture()
	s.block = &block

	var err error
	s.account, err = unittest.AccountFixture()
	s.Require().NoError(err)

	s.failingAddress = unittest.AddressFixture()
}

func (s *BackendAccountsSuite) defaultBackend() *backendAccounts {
	return &backendAccounts{
		log:              s.log,
		state:            s.state,
		headers:          s.headers,
		connFactory:      s.connectionFactory,
		nodeCommunicator: NewNodeCommunicator(false),
		execNodeIdentitiesProvider: commonrpc.NewExecutionNodeIdentitiesProvider(
			s.log,
			s.state,
			s.receipts,
			flow.IdentifierList{},
			flow.IdentifierList{},
		),
	}
}

// setupExecutionNodes sets up the mocks required to test against an EN backend
func (s *BackendAccountsSuite) setupExecutionNodes(block *flow.Block) {
	s.params.On("FinalizedRoot").Return(s.rootHeader, nil)
	s.state.On("Params").Return(s.params)
	s.state.On("Final").Return(s.snapshot)
	s.snapshot.On("Identities", mock.Anything).Return(s.executionNodes, nil)

	// this line causes a S1021 lint error because receipts is explicitly declared. this is required
	// to ensure the mock library handles the response type correctly
	var receipts flow.ExecutionReceiptList //nolint:gosimple
	receipts = unittest.ReceiptsForBlockFixture(block, s.executionNodes.NodeIDs())
	s.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)

	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil)
}

// setupENSuccessResponse configures the execution node client to return a successful response
func (s *BackendAccountsSuite) setupENSuccessResponse(blockID flow.Identifier) {
	expectedExecRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.account.Address.Bytes(),
	}

	convertedAccount, err := convert.AccountToMessage(s.account)
	s.Require().NoError(err)

	s.execClient.On("GetAccountAtBlockID", mock.Anything, expectedExecRequest).
		Return(&execproto.GetAccountAtBlockIDResponse{
			Account: convertedAccount,
		}, nil)
}

// setupENFailingResponse configures the execution node client to return an error
func (s *BackendAccountsSuite) setupENFailingResponse(blockID flow.Identifier, err error) {
	failingRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.failingAddress.Bytes(),
	}

	s.execClient.On("GetAccountAtBlockID", mock.Anything, failingRequest).
		Return(nil, err)
}

// TestGetAccountFromExecutionNode_HappyPath tests successfully getting accounts from execution nodes
func (s *BackendAccountsSuite) TestGetAccountFromExecutionNode_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeExecutionNodesOnly

	s.Run("GetAccount - happy path", func() {
		s.testGetAccount(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtLatestBlock - happy path", func() {
		s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtBlockHeight - happy path", func() {
		s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
	})
}

// TestGetAccountFromExecutionNode_Fails errors received from execution nodes are returned
func (s *BackendAccountsSuite) TestGetAccountFromExecutionNode_Fails() {
	ctx := context.Background()

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	s.setupExecutionNodes(s.block)
	s.setupENFailingResponse(s.block.ID(), errToReturn)

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeExecutionNodesOnly

	s.Run("GetAccount - fails with backend err", func() {
		s.testGetAccount(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtLatestBlock - fails with backend err", func() {
		s.testGetAccountAtLatestBlock(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtBlockHeight - fails with backend err", func() {
		s.testGetAccountAtBlockHeight(ctx, backend, statusCode)
	})
}

// TestGetAccountFromStorage_HappyPath test successfully getting accounts from local storage
func (s *BackendAccountsSuite) TestGetAccountFromStorage_HappyPath() {
	ctx := context.Background()

	scriptExecutor := execmock.NewScriptExecutor(s.T())
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Header.Height).
		Return(s.account, nil)

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	s.Run("GetAccount - happy path", func() {
		s.testGetAccount(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtLatestBlock - happy path", func() {
		s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtBlockHeight - happy path", func() {
		s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
	})
}

// TestGetAccountFromStorage_Fails tests that errors received from local storage are handled
// and converted to the appropriate status code
func (s *BackendAccountsSuite) TestGetAccountFromStorage_Fails() {
	ctx := context.Background()

	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	testCases := []struct {
		err        error
		statusCode codes.Code
	}{
		{
			err:        storage.ErrHeightNotIndexed,
			statusCode: codes.OutOfRange,
		},
		{
			err:        storage.ErrNotFound,
			statusCode: codes.NotFound,
		},
		{
			err:        fmt.Errorf("system error"),
			statusCode: codes.Internal,
		},
	}

	for _, tt := range testCases {
		scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Header.Height).
			Return(nil, tt.err).Times(3)

		s.state.On("Params").Return(s.params).Times(3)

		s.Run(fmt.Sprintf("GetAccount - fails with %v", tt.err), func() {
			s.testGetAccount(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - fails with %v", tt.err), func() {
			s.testGetAccountAtLatestBlock(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - fails with %v", tt.err), func() {
			s.params.On("SporkRootBlockHeight").Return(s.block.Header.Height-10, nil)
			s.params.On("SealedRoot").Return(s.block.Header, nil)

			s.testGetAccountAtBlockHeight(ctx, backend, tt.statusCode)
		})
	}
}

// TestGetAccountFromFailover_HappyPath tests that when an error is returned getting an account
// from local storage, the backend will attempt to get the account from an execution node
func (s *BackendAccountsSuite) TestGetAccountFromFailover_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeFailover
	backend.scriptExecutor = scriptExecutor

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Header.Height).
			Return(nil, errToReturn).Times(3)

		s.Run(fmt.Sprintf("GetAccount - happy path - recovers %v", errToReturn), func() {
			s.testGetAccount(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - happy path - recovers %v", errToReturn), func() {
			s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - happy path - recovers %v", errToReturn), func() {
			s.params.On("SporkRootBlockHeight").Return(s.block.Header.Height-10, nil)
			s.params.On("SealedRoot").Return(s.block.Header, nil)

			s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
		})
	}
}

// TestGetAccountFromFailover_ReturnsENErrors tests that when an error is returned from the execution
// node during a failover, it is returned to the caller.
func (s *BackendAccountsSuite) TestGetAccountFromFailover_ReturnsENErrors() {
	ctx := context.Background()

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	s.setupExecutionNodes(s.block)
	s.setupENFailingResponse(s.block.ID(), errToReturn)

	scriptExecutor := execmock.NewScriptExecutor(s.T())
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Header.Height).
		Return(nil, storage.ErrHeightNotIndexed)

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeFailover
	backend.scriptExecutor = scriptExecutor

	s.Run("GetAccount - fails with backend err", func() {
		s.testGetAccount(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtLatestBlock - fails with backend err", func() {
		s.testGetAccountAtLatestBlock(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtBlockHeight - fails with backend err", func() {
		s.testGetAccountAtBlockHeight(ctx, backend, statusCode)
	})
}

// TestGetAccountAtLatestBlock_InconsistentState tests that signaler context received error when node state is
// inconsistent
func (s *BackendAccountsSuite) TestGetAccountAtLatestBlockFromStorage_InconsistentState() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	s.Run(fmt.Sprintf("GetAccountAtLatestBlock - fails with %v", "inconsistent node's state"), func() {
		s.state.On("Sealed").Return(s.snapshot, nil)

		err := fmt.Errorf("inconsistent node's state")
		s.snapshot.On("Head").Return(nil, err)

		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), signCtxErr))

		actual, err := backend.GetAccountAtLatestBlock(signalerCtx, s.failingAddress)
		s.Require().Error(err)
		s.Require().Nil(actual)
	})
}

// TestGetAccountBalanceFromStorage_HappyPaths tests successfully getting accounts balance from storage
func (s *BackendAccountsSuite) TestGetAccountBalanceFromStorage_HappyPath() {
	ctx := context.Background()

	scriptExecutor := execmock.NewScriptExecutor(s.T())
	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Header.Height).
		Return(s.account.Balance, nil)

	s.Run("GetAccountBalanceAtLatestBlock - happy path", func() {
		s.testGetAccountBalanceAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountBalanceAtBlockHeight - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()
		s.testGetAccountBalanceAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountBalanceFromExecutionNode_HappyPath tests successfully getting accounts balance from execution nodes
func (s *BackendAccountsSuite) TestGetAccountBalanceFromExecutionNode_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeExecutionNodesOnly

	s.Run("GetAccountBalanceAtLatestBlock - happy path", func() {
		s.testGetAccountBalanceAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountBalanceAtBlockHeight - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()
		s.testGetAccountBalanceAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountBalanceFromFailover_HappyPath tests that when an error is returned getting accounts balance
// from local storage,  the backend will attempt to get the account balances from an execution node
func (s *BackendAccountsSuite) TestGetAccountBalanceFromFailover_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeFailover
	backend.scriptExecutor = scriptExecutor

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Header.Height).
			Return(uint64(0), errToReturn).Times(2)

		s.Run(fmt.Sprintf("GetAccountBalanceAtLatestBlock - happy path -recovers %v", errToReturn), func() {
			s.testGetAccountBalanceAtLatestBlock(ctx, backend)
		})

		s.Run(fmt.Sprintf("GetAccountBalanceAtBlockHeight - happy path -recovers %v", errToReturn), func() {
			s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()
			s.testGetAccountBalanceAtBlockHeight(ctx, backend)
		})

	}
}

// TestGetAccountKeysScriptExecutionEnabled_HappyPath tests successfully getting accounts keys when
// script execution is enabled.
func (s *BackendAccountsSuite) TestGetAccountKeysFromStorage_HappyPath() {
	ctx := context.Background()

	scriptExecutor := execmock.NewScriptExecutor(s.T())
	scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Header.Height).
		Return(s.account.Keys, nil)

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	s.Run("GetAccountKeysAtLatestBlock - happy path", func() {
		s.testGetAccountKeysAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountKeysAtBlockHeight - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

		s.testGetAccountKeysAtBlockHeight(ctx, backend)

	})
}

// TestGetAccountKeyScriptExecutionEnabled_HappyPath tests successfully getting account key by key index when
// script execution is enabled.
func (s *BackendAccountsSuite) TestGetAccountKeyFromStorage_HappyPath() {
	ctx := context.Background()
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeLocalOnly
	backend.scriptExecutor = scriptExecutor

	var keyIndex uint32 = 0
	keyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)
	scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Header.Height).
		Return(keyByIndex, nil).Twice()

	s.Run("GetAccountKeyAtLatestBlock - by key index - happy path", func() {
		s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
	})

	s.Run("GetAccountKeyAtBlockHeight - by key index - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

		s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
	})
}

// TestGetAccountKeysFromExecutionNode_HappyPath tests successfully getting accounts keys from execution nodes
func (s *BackendAccountsSuite) TestGetAccountKeysFromExecutionNode_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeExecutionNodesOnly

	s.Run("GetAccountKeysAtLatestBlock - all keys - happy path", func() {
		s.testGetAccountKeysAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountKeysAtBlockHeight - all keys - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

		s.testGetAccountKeysAtBlockHeight(ctx, backend)
	})

}

// TestGetAccountKeyFromExecutionNode_HappyPath tests successfully getting accounts key by key index from execution nodes
func (s *BackendAccountsSuite) TestGetAccountKeyFromExecutionNode_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeExecutionNodesOnly
	var keyIndex uint32 = 0

	s.Run("GetAccountKeysAtLatestBlock - by key index - happy path", func() {

		s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
	})

	s.Run("GetAccountKeysAtLatestBlock - by key index - happy path", func() {
		s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

		s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
	})
}

// TestGetAccountBalanceFromFailover_HappyPath tests that when an error is returned getting accounts keys
// from local storage, the backend will attempt to get the account key from an execution node
func (s *BackendAccountsSuite) TestGetAccountKeysFromFailover_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeFailover
	backend.scriptExecutor = scriptExecutor

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Header.Height).
			Return(nil, errToReturn).Times(2)

		s.Run(fmt.Sprintf("testGetAccountKeysAtLatestBlock -all keys - happy path - recovers %v", errToReturn), func() {
			s.testGetAccountKeysAtLatestBlock(ctx, backend)
		})

		s.Run(fmt.Sprintf("GetAccountKeysAtBlockHeight - all keys - happy path - recovers %v", errToReturn), func() {
			s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

			s.testGetAccountKeysAtBlockHeight(ctx, backend)
		})
	}
}

// TestGetAccountKeyFromFailover_HappyPath tests that when an error is returned getting account key by key index
// from local storage, the backend will attempt to get the account key from an execution node
func (s *BackendAccountsSuite) TestGetAccountKeyFromFailover_HappyPath() {
	ctx := context.Background()

	s.setupExecutionNodes(s.block)
	s.setupENSuccessResponse(s.block.ID())

	scriptExecutor := execmock.NewScriptExecutor(s.T())

	backend := s.defaultBackend()
	backend.scriptExecMode = IndexQueryModeFailover
	backend.scriptExecutor = scriptExecutor

	var keyIndex uint32 = 0

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Header.Height).
			Return(nil, errToReturn).Times(2)

		s.Run(fmt.Sprintf("testGetAccountKeysAtLatestBlock - by key index - happy path - recovers %v", errToReturn), func() {
			s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
		})

		s.Run(fmt.Sprintf("GetAccountKeysAtBlockHeight - by key index - happy path - recovers %v", errToReturn), func() {
			s.headers.On("BlockIDByHeight", s.block.Header.Height).Return(s.block.Header.ID(), nil).Once()

			s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
		})
	}
}

func (s *BackendAccountsSuite) testGetAccount(ctx context.Context, backend *backendAccounts, statusCode codes.Code) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.Header, nil).Once()

	if statusCode == codes.OK {
		actual, err := backend.GetAccount(ctx, s.account.Address)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
	} else {
		actual, err := backend.GetAccount(ctx, s.failingAddress)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
	}
}

func (s *BackendAccountsSuite) testGetAccountAtLatestBlock(ctx context.Context, backend *backendAccounts, statusCode codes.Code) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.Header, nil).Once()

	if statusCode == codes.OK {
		actual, err := backend.GetAccountAtLatestBlock(ctx, s.account.Address)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
	} else {
		actual, err := backend.GetAccountAtLatestBlock(ctx, s.failingAddress)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
	}
}

func (s *BackendAccountsSuite) testGetAccountAtBlockHeight(ctx context.Context, backend *backendAccounts, statusCode codes.Code) {
	height := s.block.Header.Height
	s.headers.On("BlockIDByHeight", height).Return(s.block.Header.ID(), nil).Once()

	if statusCode == codes.OK {
		actual, err := backend.GetAccountAtBlockHeight(ctx, s.account.Address, height)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
	} else {
		actual, err := backend.GetAccountAtBlockHeight(ctx, s.failingAddress, height)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
	}
}

func (s *BackendAccountsSuite) testGetAccountBalanceAtLatestBlock(ctx context.Context, backend *backendAccounts) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.Header, nil).Once()

	actual, err := backend.GetAccountBalanceAtLatestBlock(ctx, s.account.Address)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Balance, actual)
}

func (s *BackendAccountsSuite) testGetAccountBalanceAtBlockHeight(ctx context.Context, backend *backendAccounts) {
	actual, err := backend.GetAccountBalanceAtBlockHeight(ctx, s.account.Address, s.block.Header.Height)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Balance, actual)
}

func (s *BackendAccountsSuite) testGetAccountKeysAtLatestBlock(ctx context.Context, backend *backendAccounts) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.Header, nil).Once()

	actual, err := backend.GetAccountKeysAtLatestBlock(ctx, s.account.Address)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Keys, actual)
}

func (s *BackendAccountsSuite) testGetAccountKeyAtLatestBlock(ctx context.Context, backend *backendAccounts, keyIndex uint32) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.Header, nil).Once()

	actual, err := backend.GetAccountKeyAtLatestBlock(ctx, s.account.Address, keyIndex)
	expectedKeyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)
	s.Require().NoError(err)
	s.Require().Equal(expectedKeyByIndex, actual)
}

func (s *BackendAccountsSuite) testGetAccountKeysAtBlockHeight(ctx context.Context, backend *backendAccounts) {
	actual, err := backend.GetAccountKeysAtBlockHeight(ctx, s.account.Address, s.block.Header.Height)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Keys, actual)
}

func (s *BackendAccountsSuite) testGetAccountKeyAtBlockHeight(ctx context.Context, backend *backendAccounts, keyIndex uint32) {

	actual, err := backend.GetAccountKeyAtBlockHeight(ctx, s.account.Address, keyIndex, s.block.Header.Height)
	expectedKeyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)
	s.Require().NoError(err)
	s.Require().Equal(expectedKeyByIndex, actual)
}

func findAccountKeyByIndex(keys []flow.AccountPublicKey, keyIndex uint32) *flow.AccountPublicKey {
	for _, key := range keys {
		if key.Index == keyIndex {
			return &key
		}
	}
	return &flow.AccountPublicKey{}
}
