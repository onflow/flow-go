package backend

import (
	"context"
	"fmt"
	"testing"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	access "github.com/onflow/flow-go/engine/access/mock"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
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
		log:               s.log,
		state:             s.state,
		headers:           s.headers,
		executionReceipts: s.receipts,
		connFactory:       s.connectionFactory,
		nodeCommunicator:  NewNodeCommunicator(false),
	}
}

// setupExecutionNodes sets up the mocks required to test against an EN backend
func (s *BackendAccountsSuite) setupExecutionNodes(block *flow.Block) {
	s.params.On("FinalizedRoot").Return(s.rootHeader, nil)
	s.state.On("Params").Return(s.params)
	s.state.On("Final").Return(s.snapshot)
	s.snapshot.On("Identities", mock.Anything).Return(s.executionNodes, nil)

	var receipts flow.ExecutionReceiptList
	receipts = unittest.ReceiptsForBlockFixture(block, s.executionNodes.NodeIDs())
	s.receipts.On("ByBlockID", block.ID()).Return(receipts, nil)

	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mockCloser{}, nil)

	// create the expected execution API request
	blockID := block.ID()
	expectedExecRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.account.Address.Bytes(),
	}

	// create the expected execution API response
	convertedAccount, err := convert.AccountToMessage(s.account)
	s.Require().NoError(err)

	s.execClient.On("GetAccountAtBlockID", mock.Anything, expectedExecRequest).
		Return(&execproto.GetAccountAtBlockIDResponse{
			Account: convertedAccount,
		}, nil)
}

func (s *BackendAccountsSuite) TestGetAccountFromExecutionNode() {
	ctx := context.Background()

	// setup the execution client mocks
	s.setupExecutionNodes(s.block)

	backend := s.defaultBackend()
	backend.scriptExecMode = ScriptExecutionModeExecutionNodesOnly

	s.Run("GetAccount - happy path", func() {
		s.testGetAccount(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtLatestBlock - happy path", func() {
		s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtBlockHeight - happy path", func() {
		s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
	})

	// test EN errors are passed through
	blockID := s.block.ID()
	failingRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.failingAddress.Bytes(),
	}

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	s.execClient.On("GetAccountAtBlockID", mock.Anything, failingRequest).
		Return(nil, errToReturn).Times(6) // one set for each of the 2 ENs

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

func (s *BackendAccountsSuite) TestGetAccountFromStorage() {
	ctx := context.Background()

	scriptExecutor := execmock.NewScriptExecutor(s.T())
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Header.Height).
		Return(s.account, nil)

	backend := s.defaultBackend()
	backend.scriptExecMode = ScriptExecutionModeLocalOnly
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

	// run tests where the following errors are returned from the script executor and verify the
	// expected grpc status code is returned
	errors := []error{
		ErrDataNotAvailable,
		storage.ErrNotFound,
		fmt.Errorf("random error"),
	}
	for _, errToReturn := range errors {
		var statusCode codes.Code
		switch errToReturn {
		case ErrDataNotAvailable:
			statusCode = codes.OutOfRange
		case storage.ErrNotFound:
			statusCode = codes.NotFound
		default:
			statusCode = codes.Internal
		}

		scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Header.Height).
			Return(nil, errToReturn).Times(3)

		s.Run(fmt.Sprintf("GetAccount - fails with %v", errToReturn), func() {
			s.testGetAccount(ctx, backend, statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - fails with %v", errToReturn), func() {
			s.testGetAccountAtLatestBlock(ctx, backend, statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - fails with %v", errToReturn), func() {
			s.testGetAccountAtBlockHeight(ctx, backend, statusCode)
		})
	}
}

func (s *BackendAccountsSuite) TestGetAccountFromFailover() {
	ctx := context.Background()

	for _, errToReturn := range []error{ErrDataNotAvailable, storage.ErrNotFound} {
		// configure local script executor to fail
		scriptExecutor := execmock.NewScriptExecutor(s.T())
		scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Header.Height).
			Return(nil, errToReturn)

		// setup the execution client mocks
		s.setupExecutionNodes(s.block)

		backend := s.defaultBackend()
		backend.scriptExecMode = ScriptExecutionModeFailover
		backend.scriptExecutor = scriptExecutor

		s.Run(fmt.Sprintf("GetAccount - happy path - recovers %v", errToReturn), func() {
			s.testGetAccount(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - happy path - recovers %v", errToReturn), func() {
			s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - happy path - recovers %v", errToReturn), func() {
			s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
		})
	}

	// Finally, test when the EN fails as well

	// configure local script executor to fail
	scriptExecutor := execmock.NewScriptExecutor(s.T())
	scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Header.Height).
		Return(nil, ErrDataNotAvailable)

	// setup the execution client mocks
	s.setupExecutionNodes(s.block)

	backend := s.defaultBackend()
	backend.scriptExecMode = ScriptExecutionModeFailover
	backend.scriptExecutor = scriptExecutor

	// use a status code that's not used in the API to make sure it's passed through
	// test EN errors are passed through
	blockID := s.block.ID()
	failingRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.failingAddress.Bytes(),
	}

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	s.execClient.On("GetAccountAtBlockID", mock.Anything, failingRequest).
		Return(nil, errToReturn).Times(6) // one set for each of the 2 ENs

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
	s.headers.On("ByHeight", height).Return(s.block.Header, nil).Once()

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
