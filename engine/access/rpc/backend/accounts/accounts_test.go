package accounts

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	access "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
)

type AccountsSuite struct {
	suite.Suite

	log        zerolog.Logger
	state      *protocol.State
	snapshot   *protocol.Snapshot
	params     *protocol.Params
	rootHeader *flow.Header

	headers           *storagemock.Headers
	receipts          *storagemock.ExecutionReceipts
	registers         *storagemock.RegisterSnapshotReader
	connectionFactory *connectionmock.ConnectionFactory
	chainID           flow.ChainID

	executionNodes flow.IdentityList
	execClient     *access.ExecutionAPIClient

	block               *flow.Block
	account             *flow.Account
	failingAddress      flow.Address
	executionResult     *flow.ExecutionResult
	executionResultInfo *optimistic_sync.ExecutionResultInfo

	executionResultProvider *osyncmock.ExecutionResultProvider
	executionStateCache     *osyncmock.ExecutionStateCache
	scriptExecutor          *execmock.ScriptExecutor
	executionDataSnapshot   *osyncmock.Snapshot
	criteria                optimistic_sync.Criteria
	expectedMetadata        *accessmodel.ExecutorMetadata
}

func TestBackendAccountsSuite(t *testing.T) {
	suite.Run(t, new(AccountsSuite))
}

func (s *AccountsSuite) SetupTest() {
	s.log = unittest.Logger()
	s.state = protocol.NewState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.rootHeader = unittest.BlockHeaderFixture()
	s.params = protocol.NewParams(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.registers = storagemock.NewRegisterSnapshotReader(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.chainID = flow.Testnet

	s.execClient = access.NewExecutionAPIClient(s.T())
	s.executionNodes = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
	s.block = unittest.BlockFixture()
	s.executionResult = unittest.ExecutionResultFixture()
	s.executionResultInfo = &optimistic_sync.ExecutionResultInfo{
		ExecutionResultID: s.executionResult.ID(),
		ExecutionNodes:    s.executionNodes.ToSkeleton(),
	}
	s.criteria = optimistic_sync.Criteria{}
	s.executionResultProvider = osyncmock.NewExecutionResultProvider(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.scriptExecutor = execmock.NewScriptExecutor(s.T())

	var err error
	s.account, err = unittest.AccountFixture()
	s.Require().NoError(err)

	s.failingAddress = unittest.AddressFixture()
	s.expectedMetadata = &accessmodel.ExecutorMetadata{
		ExecutionResultID: s.executionResult.ID(),
		ExecutorIDs:       s.executionNodes.NodeIDs(),
	}
}

// TestGetAccountFromExecutionNode_HappyPath tests successfully getting accounts from execution nodes
func (s *AccountsSuite) TestGetAccountFromExecutionNode_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("GetAccount - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccount(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtLatestBlock - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtBlockHeight - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
	})
}

// TestGetAccountFromExecutionNode_Fails errors received from execution nodes are returned
func (s *AccountsSuite) TestGetAccountFromExecutionNode_Fails() {
	ctx := context.Background()
	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("GetAccount - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccount(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtLatestBlock - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccountAtLatestBlock(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtBlockHeight - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccountAtBlockHeight(ctx, backend, statusCode)
	})
}

// TestGetAccountFromStorage_HappyPath test successfully getting accounts from local storage
func (s *AccountsSuite) TestGetAccountFromStorage_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run("GetAccount - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account, nil).
			Once()

		s.testGetAccount(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtLatestBlock - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account, nil).
			Once()

		s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
	})

	s.Run("GetAccountAtBlockHeight - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account, nil).
			Once()

		s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
	})
}

// TestGetAccountFromStorage_Fails tests that errors received from local storage are handled
// and converted to the appropriate status code
func (s *AccountsSuite) TestGetAccountFromStorage_Fails() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

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
		s.Run(fmt.Sprintf("GetAccount - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()
			// used in error conversion, when local provider fails
			s.mockGetAccountErrorConversionFromStorage(tt.err)

			s.testGetAccount(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()
			// used in error conversion, when local provider fails
			s.mockGetAccountErrorConversionFromStorage(tt.err)

			s.testGetAccountAtLatestBlock(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()
			// used in error conversion, when local provider fails
			s.mockGetAccountErrorConversionFromStorage(tt.err)

			s.testGetAccountAtBlockHeight(ctx, backend, tt.statusCode)
		})
	}
}

func (s *AccountsSuite) mockGetAccountErrorConversionFromStorage(err error) {
	s.state.On("Params").Return(s.params).Once()

	if errors.Is(err, storage.ErrNotFound) {
		s.params.On("SporkRootBlockHeight").Return(s.block.Height-10, nil).Once()
		s.params.On("SealedRoot").Return(s.block.ToHeader(), nil).Once()
	}
}

// TestGetAccountFromFailover_HappyPath tests that when an error is returned getting an account
// from local storage, the backend will attempt to get the account from an execution node
func (s *AccountsSuite) TestGetAccountFromFailover_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeFailover)

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		s.Run(fmt.Sprintf("GetAccount - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			// used in error conversion
			s.mockGetAccountErrorConversionFromStorage(errToReturn)
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccount(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtLatestBlock - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			// used in error conversion
			s.mockGetAccountErrorConversionFromStorage(errToReturn)
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountAtLatestBlock(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("GetAccountAtBlockHeight - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			// used in error conversion, when local provider fails
			s.mockGetAccountErrorConversionFromStorage(errToReturn)
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountAtBlockHeight(ctx, backend, codes.OK)
		})
	}
}

// TestGetAccountFromFailover_ReturnsENErrors tests that when an error is returned from the execution
// node during a failover, it is returned to the caller.
func (s *AccountsSuite) TestGetAccountFromFailover_ReturnsENErrors() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeFailover)

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	// used in error conversion
	s.state.On("Params").Return(s.params)

	s.Run("GetAccount - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccount(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtLatestBlock - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccountAtLatestBlock(ctx, backend, statusCode)
	})

	s.Run("GetAccountAtBlockHeight - fails with backend err", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountAtBlockHeight", mock.Anything, s.failingAddress, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testGetAccountAtBlockHeight(ctx, backend, statusCode)
	})
}

// TestGetAccountAtLatestBlock_InconsistentState tests that signaler context received error when node state is
// inconsistent
func (s *AccountsSuite) TestGetAccountAtLatestBlockFromStorage_InconsistentState() {
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run(fmt.Sprintf("GetAccountAtLatestBlock - fails with %v", "inconsistent node state"), func() {
		s.state.On("Sealed").Return(s.snapshot, nil).Once()

		err := fmt.Errorf("inconsistent node state")
		s.snapshot.On("Head").Return(nil, err).Once()

		signCtxErr := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		signalerCtx := irrecoverable.WithSignalerContext(context.Background(), irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), signCtxErr))

		actual, metadata, err := backend.GetAccountAtLatestBlock(signalerCtx, s.failingAddress, s.criteria)
		s.Require().Error(err)
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	})
}

// TestGetAccountBalanceFromStorage_HappyPaths tests successfully getting accounts balance from storage
func (s *AccountsSuite) TestGetAccountBalanceFromStorage_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run("GetAccountBalanceAtLatestBlock - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account.Balance, nil).
			Once()

		s.testGetAccountBalanceAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountBalanceAtBlockHeight - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account.Balance, nil).
			Once()
		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
		s.testGetAccountBalanceAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountBalanceFromExecutionNode_HappyPath tests successfully getting accounts balance from execution nodes
func (s *AccountsSuite) TestGetAccountBalanceFromExecutionNode_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("GetAccountBalanceAtLatestBlock - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountBalanceAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountBalanceAtBlockHeight - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountBalanceAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountBalanceFromFailover_HappyPath tests that when an error is returned getting accounts balance
// from local storage,  the backend will attempt to get the account balances from an execution node
func (s *AccountsSuite) TestGetAccountBalanceFromFailover_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeFailover)

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		s.Run(fmt.Sprintf("GetAccountBalanceAtLatestBlock - happy path -recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			// configure local script executor to fail
			s.scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(uint64(0), errToReturn).
				Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountBalanceAtLatestBlock(ctx, backend)
		})

		s.Run(fmt.Sprintf("GetAccountBalanceAtBlockHeight - happy path -recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			// configure local script executor to fail
			s.scriptExecutor.On("GetAccountBalance", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(uint64(0), errToReturn).
				Once()
			s.setupENSuccessResponse(s.block.ID())
			s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()

			s.testGetAccountBalanceAtBlockHeight(ctx, backend)
		})

	}
}

// TestGetAccountKeysScriptExecutionEnabled_HappyPath tests successfully getting accounts keys when
// script execution is enabled.
func (s *AccountsSuite) TestGetAccountKeysFromStorage_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run("GetAccountKeysAtLatestBlock - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account.Keys, nil).
			Once()
		s.testGetAccountKeysAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountKeysAtBlockHeight - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Height, s.registers).
			Return(s.account.Keys, nil).
			Once()
		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()

		s.testGetAccountKeysAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountKeyScriptExecutionEnabled_HappyPath tests successfully getting account key by key index when
// script execution is enabled.
func (s *AccountsSuite) TestGetAccountKeyFromStorage_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeLocalOnly)

	var keyIndex uint32 = 0
	keyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)

	s.Run("GetAccountKeyAtLatestBlock - by key index - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Height, s.registers).
			Return(keyByIndex, nil).
			Once()

		s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
	})

	s.Run("GetAccountKeyAtBlockHeight - by key index - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.executionStateCache.
			On("Snapshot", s.executionResult.ID()).
			Return(s.executionDataSnapshot, nil).
			Once()
		s.executionDataSnapshot.
			On("Registers").
			Return(s.registers, nil).
			Once()
		s.scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Height, s.registers).
			Return(keyByIndex, nil).
			Once()

		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()

		s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
	})
}

// TestGetAccountKeysFromExecutionNode_HappyPath tests successfully getting accounts keys from execution nodes
func (s *AccountsSuite) TestGetAccountKeysFromExecutionNode_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("GetAccountKeysAtLatestBlock - all keys - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountKeysAtLatestBlock(ctx, backend)
	})

	s.Run("GetAccountKeysAtBlockHeight - all keys - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountKeysAtBlockHeight(ctx, backend)
	})
}

// TestGetAccountKeyFromExecutionNode_HappyPath tests successfully getting accounts key by key index from execution nodes
func (s *AccountsSuite) TestGetAccountKeyFromExecutionNode_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeExecutionNodesOnly)
	var keyIndex uint32 = 0

	s.Run("GetAccountKeysAtLatestBlock - by key index - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
	})

	s.Run("GetAccountKeysAtLatestBlock - by key index - happy path", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
	})
}

// TestGetAccountBalanceFromFailover_HappyPath tests that when an error is returned getting accounts keys
// from local storage, the backend will attempt to get the account key from an execution node
func (s *AccountsSuite) TestGetAccountKeysFromFailover_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeFailover)

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		s.Run(fmt.Sprintf("testGetAccountKeysAtLatestBlock -all keys - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountKeysAtLatestBlock(ctx, backend)
		})

		s.Run(fmt.Sprintf("GetAccountKeysAtBlockHeight - all keys - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			s.scriptExecutor.On("GetAccountKeys", mock.Anything, s.account.Address, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountKeysAtBlockHeight(ctx, backend)
		})
	}
}

// TestGetAccountKeyFromFailover_HappyPath tests that when an error is returned getting account key by key index
// from local storage, the backend will attempt to get the account key from an execution node
func (s *AccountsSuite) TestGetAccountKeyFromFailover_HappyPath() {
	ctx := context.Background()
	backend := s.defaultAccountsBackend(query_mode.IndexQueryModeFailover)
	var keyIndex uint32 = 0

	for _, errToReturn := range []error{storage.ErrHeightNotIndexed, storage.ErrNotFound} {
		s.Run(fmt.Sprintf("testGetAccountKeysAtLatestBlock - by key index - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			// configure local script executor to fail
			s.scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountKeyAtLatestBlock(ctx, backend, keyIndex)
		})

		s.Run(fmt.Sprintf("GetAccountKeysAtBlockHeight - by key index - happy path - recovers %v", errToReturn), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", s.executionResult.ID()).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers, nil).
				Once()
			// configure local script executor to fail
			s.scriptExecutor.On("GetAccountKey", mock.Anything, s.account.Address, keyIndex, s.block.Height, s.registers).
				Return(nil, errToReturn).
				Once()
			s.headers.On("BlockIDByHeight", s.block.Height).Return(s.block.ID(), nil).Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testGetAccountKeyAtBlockHeight(ctx, backend, keyIndex)
		})
	}
}

func (s *AccountsSuite) testGetAccount(ctx context.Context, backend *Accounts, statusCode codes.Code) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	if statusCode == codes.OK {
		actual, metadata, err := backend.GetAccount(ctx, s.account.Address, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := backend.GetAccount(ctx, s.failingAddress, s.criteria)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}
}

func (s *AccountsSuite) testGetAccountAtLatestBlock(ctx context.Context, backend *Accounts, statusCode codes.Code) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	if statusCode == codes.OK {
		actual, metadata, err := backend.GetAccountAtLatestBlock(ctx, s.account.Address, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := backend.GetAccountAtLatestBlock(ctx, s.failingAddress, s.criteria)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}
}

func (s *AccountsSuite) testGetAccountAtBlockHeight(ctx context.Context, backend *Accounts, statusCode codes.Code) {
	height := s.block.Height
	s.headers.On("BlockIDByHeight", height).Return(s.block.ID(), nil).Once()

	if statusCode == codes.OK {
		actual, metadata, err := backend.GetAccountAtBlockHeight(ctx, s.account.Address, height, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(s.account, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := backend.GetAccountAtBlockHeight(ctx, s.failingAddress, height, s.criteria)
		s.Require().Error(err)
		s.Require().Equal(statusCode, status.Code(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}
}

func (s *AccountsSuite) testGetAccountBalanceAtLatestBlock(ctx context.Context, backend *Accounts) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	actual, metadata, err := backend.GetAccountBalanceAtLatestBlock(ctx, s.account.Address, s.criteria)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Balance, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func (s *AccountsSuite) testGetAccountBalanceAtBlockHeight(ctx context.Context, backend *Accounts) {
	actual, metadata, err := backend.GetAccountBalanceAtBlockHeight(ctx, s.account.Address, s.block.Height, s.criteria)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Balance, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func (s *AccountsSuite) testGetAccountKeysAtLatestBlock(ctx context.Context, backend *Accounts) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	actual, metadata, err := backend.GetAccountKeysAtLatestBlock(ctx, s.account.Address, s.criteria)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Keys, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func (s *AccountsSuite) testGetAccountKeyAtLatestBlock(ctx context.Context, backend *Accounts, keyIndex uint32) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	actual, metadata, err := backend.GetAccountKeyAtLatestBlock(ctx, s.account.Address, keyIndex, s.criteria)
	expectedKeyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)
	s.Require().NoError(err)
	s.Require().Equal(expectedKeyByIndex, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func (s *AccountsSuite) testGetAccountKeysAtBlockHeight(ctx context.Context, backend *Accounts) {
	actual, metadata, err := backend.GetAccountKeysAtBlockHeight(ctx, s.account.Address, s.block.Height, s.criteria)
	s.Require().NoError(err)
	s.Require().Equal(s.account.Keys, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func (s *AccountsSuite) testGetAccountKeyAtBlockHeight(ctx context.Context, backend *Accounts, keyIndex uint32) {
	actual, metadata, err := backend.GetAccountKeyAtBlockHeight(ctx, s.account.Address, keyIndex, s.block.Height, s.criteria)
	expectedKeyByIndex := findAccountKeyByIndex(s.account.Keys, keyIndex)
	s.Require().NoError(err)
	s.Require().Equal(expectedKeyByIndex, actual)
	s.Require().Equal(s.expectedMetadata, metadata)
}

func findAccountKeyByIndex(keys []flow.AccountPublicKey, keyIndex uint32) *flow.AccountPublicKey {
	for _, key := range keys {
		if key.Index == keyIndex {
			return &key
		}
	}
	return &flow.AccountPublicKey{}
}

func (s *AccountsSuite) defaultAccountsBackend(mode query_mode.IndexQueryMode) *Accounts {
	accounts, err := NewAccountsBackend(
		s.log,
		s.state,
		s.headers,
		s.connectionFactory,
		node_communicator.NewNodeCommunicator(false),
		mode,
		s.scriptExecutor,
		commonrpc.NewExecutionNodeIdentitiesProvider(
			s.log,
			s.state,
			s.receipts,
			flow.IdentifierList{},
			flow.IdentifierList{},
		),
		s.executionResultProvider,
		s.executionStateCache,
	)
	require.NoError(s.T(), err)

	return accounts
}

// setupENSuccessResponse configures the execution node client to return a successful response
func (s *AccountsSuite) setupENSuccessResponse(blockID flow.Identifier) {
	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil).
		Once()

	expectedExecRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.account.Address.Bytes(),
	}

	convertedAccount, err := convert.AccountToMessage(s.account)
	s.Require().NoError(err)

	s.execClient.On("GetAccountAtBlockID", mock.Anything, expectedExecRequest).
		Return(&execproto.GetAccountAtBlockIDResponse{
			Account: convertedAccount,
		}, nil).
		Once()
}

// setupENFailingResponse configures the execution node client to return an error
func (s *AccountsSuite) setupENFailingResponse(blockID flow.Identifier, err error) {
	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil).
		Times(len(s.executionNodes))

	failingRequest := &execproto.GetAccountAtBlockIDRequest{
		BlockId: blockID[:],
		Address: s.failingAddress.Bytes(),
	}

	s.execClient.On("GetAccountAtBlockID", mock.Anything, failingRequest).
		Return(nil, err).
		Times(len(s.executionNodes))
}
