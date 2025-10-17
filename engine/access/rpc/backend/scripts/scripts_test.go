package scripts

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/engine/access/mock"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	connectionmock "github.com/onflow/flow-go/engine/access/rpc/connection/mock"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	osyncmock "github.com/onflow/flow-go/module/executiondatasync/optimistic_sync/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

var (
	expectedResponse = []byte("response_data")

	cadenceErr    = fvmerrors.NewCodedError(fvmerrors.ErrCodeCadenceRunTimeError, "cadence error")
	fvmFailureErr = fvmerrors.NewCodedFailure(fvmerrors.FailureCodeBlockFinderFailure, "fvm error")
	ctxCancelErr  = fvmerrors.NewCodedError(fvmerrors.ErrCodeScriptExecutionCancelledError, "context canceled error")
	timeoutErr    = fvmerrors.NewCodedError(fvmerrors.ErrCodeScriptExecutionTimedOutError, "timeout error")
	compLimitErr  = fvmerrors.NewCodedError(fvmerrors.ErrCodeComputationLimitExceededError, "computation limit exceeded error")
	memLimitErr   = fvmerrors.NewCodedError(fvmerrors.ErrCodeMemoryLimitExceededError, "memory limit exceeded error")
)

// BackendScriptsSuite defines a test suite for verifying the Scripts backend behavior.
type BackendScriptsSuite struct {
	suite.Suite

	log      zerolog.Logger
	state    *protocol.State
	snapshot *protocol.Snapshot

	headers           *storagemock.Headers
	receipts          *storagemock.ExecutionReceipts
	registers         *storagemock.RegisterSnapshotReader
	connectionFactory *connectionmock.ConnectionFactory

	executionNodes      flow.IdentityList
	execClient          *accessmock.ExecutionAPIClient
	executionResult     *flow.ExecutionResult
	executionResultInfo *optimistic_sync.ExecutionResultInfo
	block               *flow.Block

	script        []byte
	arguments     [][]byte
	failingScript []byte

	executionResultProvider *osyncmock.ExecutionResultProvider
	executionStateCache     *osyncmock.ExecutionStateCache
	scriptExecutor          *execmock.ScriptExecutor
	executionDataSnapshot   *osyncmock.Snapshot
	criteria                optimistic_sync.Criteria
	expectedMetadata        *accessmodel.ExecutorMetadata
}

func TestBackendScriptsSuite(t *testing.T) {
	suite.Run(t, new(BackendScriptsSuite))
}

func (s *BackendScriptsSuite) SetupTest() {
	s.log = unittest.Logger()
	s.state = protocol.NewState(s.T())
	s.snapshot = protocol.NewSnapshot(s.T())
	s.headers = storagemock.NewHeaders(s.T())
	s.receipts = storagemock.NewExecutionReceipts(s.T())
	s.connectionFactory = connectionmock.NewConnectionFactory(s.T())
	s.executionResultProvider = osyncmock.NewExecutionResultProvider(s.T())
	s.executionStateCache = osyncmock.NewExecutionStateCache(s.T())
	s.executionDataSnapshot = osyncmock.NewSnapshot(s.T())
	s.scriptExecutor = execmock.NewScriptExecutor(s.T())

	s.execClient = accessmock.NewExecutionAPIClient(s.T())
	s.executionNodes = unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleExecution))
	s.block = unittest.BlockFixture()
	s.executionResult = unittest.ExecutionResultFixture()
	s.executionResultInfo = &optimistic_sync.ExecutionResultInfo{
		ExecutionResultID: s.executionResult.ID(),
		ExecutionNodes:    s.executionNodes.ToSkeleton(),
	}
	s.criteria = optimistic_sync.Criteria{}

	s.script = []byte("access(all) fun main() { return 1 }")
	s.arguments = [][]byte{[]byte("arg1"), []byte("arg2")}
	s.failingScript = []byte("access(all) fun main() { panic(\"!!\") }")
	s.expectedMetadata = &accessmodel.ExecutorMetadata{
		ExecutionResultID: s.executionResult.ID(),
		ExecutorIDs:       s.executionNodes.NodeIDs(),
	}
}

// defaultBackend creates a new Scripts backend configured for the given query mode.
func (s *BackendScriptsSuite) defaultBackend(mode query_mode.IndexQueryMode) *Scripts {
	loggedScripts, err := lru.New[[md5.Size]byte, time.Time](common.DefaultLoggedScriptsCacheSize)
	s.Require().NoError(err)

	scripts, err := NewScriptsBackend(
		s.log,
		metrics.NewNoopCollector(),
		s.headers,
		s.state,
		s.connectionFactory,
		node_communicator.NewNodeCommunicator(false),
		s.scriptExecutor,
		mode,
		commonrpc.NewExecutionNodeIdentitiesProvider(
			s.log,
			s.state,
			s.receipts,
			flow.IdentifierList{},
			flow.IdentifierList{},
		),
		loggedScripts,
		commonrpc.DefaultAccessMaxRequestSize,
		s.executionResultProvider,
		s.executionStateCache,
	)
	require.NoError(s.T(), err)

	return scripts
}

// setupENSuccessResponse configures the execution client mock to return a successful response.
func (s *BackendScriptsSuite) setupENSuccessResponse(blockID flow.Identifier) {
	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil).
		Once()

	expectedExecRequest := &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    s.script,
		Arguments: s.arguments,
	}

	s.execClient.On("ExecuteScriptAtBlockID", mock.Anything, expectedExecRequest).
		Return(&execproto.ExecuteScriptAtBlockIDResponse{
			Value: expectedResponse,
		}, nil).
		Once()
}

// setupENFailingResponse configures the execution client mock to return a failing response.
func (s *BackendScriptsSuite) setupENFailingResponse(blockID flow.Identifier, err error) {
	s.connectionFactory.On("GetExecutionAPIClient", mock.Anything).
		Return(s.execClient, &mocks.MockCloser{}, nil).
		Times(len(s.executionNodes))

	expectedExecRequest := &execproto.ExecuteScriptAtBlockIDRequest{
		BlockId:   blockID[:],
		Script:    s.failingScript,
		Arguments: s.arguments,
	}

	s.execClient.On("ExecuteScriptAtBlockID", mock.Anything, expectedExecRequest).
		Return(nil, err).
		Times(len(s.executionNodes))
}

// TestExecuteScriptOnExecutionNode_HappyPath tests that the backend successfully executes scripts
// on execution nodes.
func (s *BackendScriptsSuite) TestExecuteScriptOnExecutionNode_HappyPath() {
	ctx := context.Background()
	scripts := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("ExecuteScriptAtLatestBlock", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testExecuteScriptAtLatestBlock(ctx, scripts, codes.OK)
	})

	s.Run("ExecuteScriptAtBlockID", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testExecuteScriptAtBlockID(ctx, scripts, codes.OK)
	})

	s.Run("ExecuteScriptAtBlockHeight", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENSuccessResponse(s.block.ID())

		s.testExecuteScriptAtBlockHeight(ctx, scripts, codes.OK)
	})

	s.executionResultProvider.AssertExpectations(s.T())
	s.connectionFactory.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}

// TestExecuteScriptOnExecutionNode_Fails tests that the backend returns an error when the execution
// node returns an error.
func (s *BackendScriptsSuite) TestExecuteScriptOnExecutionNode_Fails() {
	ctx := context.Background()

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	scripts := s.defaultBackend(query_mode.IndexQueryModeExecutionNodesOnly)

	s.Run("ExecuteScriptAtLatestBlock", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtLatestBlock(ctx, scripts, statusCode)
	})

	s.Run("ExecuteScriptAtBlockID", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtBlockID(ctx, scripts, statusCode)
	})

	s.Run("ExecuteScriptAtBlockHeight", func() {
		s.executionResultProvider.
			On("ExecutionResultInfo", s.block.ID(), s.criteria).
			Return(s.executionResultInfo, nil).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtBlockHeight(ctx, scripts, statusCode)
	})

	s.executionResultProvider.AssertExpectations(s.T())
	s.connectionFactory.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}

// TestExecuteScriptFromStorage_HappyPath tests that the backend successfully executes scripts using
// the local storage.
func (s *BackendScriptsSuite) TestExecuteScriptFromStorage_HappyPath() {
	ctx := context.Background()
	scripts := s.defaultBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run("ExecuteScriptAtLatestBlock - happy path", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
			Return(expectedResponse, nil).
			Once()

		s.testExecuteScriptAtLatestBlock(ctx, scripts, codes.OK)
	})

	s.Run("ExecuteScriptAtBlockID - happy path", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
			Return(expectedResponse, nil).
			Once()

		s.testExecuteScriptAtBlockID(ctx, scripts, codes.OK)
	})

	s.Run("ExecuteScriptAtBlockHeight - happy path", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
			Return(expectedResponse, nil).
			Once()

		s.testExecuteScriptAtBlockHeight(ctx, scripts, codes.OK)
	})

	s.executionResultProvider.AssertExpectations(s.T())
	s.executionStateCache.AssertExpectations(s.T())
	s.executionDataSnapshot.AssertExpectations(s.T())
	s.scriptExecutor.AssertExpectations(s.T())
}

// TestExecuteScriptFromStorage_Fails tests that errors received from local storage are handled
// and converted to the appropriate status code..
func (s *BackendScriptsSuite) TestExecuteScriptFromStorage_Fails() {
	ctx := context.Background()

	backend := s.defaultBackend(query_mode.IndexQueryModeLocalOnly)

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
		{
			err:        cadenceErr,
			statusCode: codes.InvalidArgument,
		},
		{
			err:        fvmFailureErr,
			statusCode: codes.Internal,
		},
	}

	for _, tt := range testCases {
		s.Run(fmt.Sprintf("ExecuteScriptAtLatestBlock - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", mock.Anything).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers).
				Once()
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).Once()

			s.testExecuteScriptAtLatestBlock(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockID - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", mock.Anything).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers).
				Once()
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).Once()

			s.testExecuteScriptAtBlockID(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockHeight - fails with %v", tt.err), func() {
			s.executionResultProvider.
				On("ExecutionResultInfo", s.block.ID(), s.criteria).
				Return(s.executionResultInfo, nil).
				Once()
			s.executionStateCache.
				On("Snapshot", mock.Anything).
				Return(s.executionDataSnapshot, nil).
				Once()
			s.executionDataSnapshot.
				On("Registers").
				Return(s.registers).
				Once()
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).Once()

			s.testExecuteScriptAtBlockHeight(ctx, backend, tt.statusCode)
		})
	}

	s.executionResultProvider.AssertExpectations(s.T())
	s.executionStateCache.AssertExpectations(s.T())
	s.executionDataSnapshot.AssertExpectations(s.T())
	s.scriptExecutor.AssertExpectations(s.T())
}

// TestExecuteScriptWithFailover_HappyPath tests that when an error is returned executing a script
// from local storage, the backend will attempt to run it on an execution node
func (s *BackendScriptsSuite) TestExecuteScriptWithFailover_HappyPath() {
	ctx := context.Background()

	errors := []error{
		storage.ErrHeightNotIndexed,
		storage.ErrNotFound,
		fmt.Errorf("system error"),
		fvmFailureErr,
		compLimitErr,
		memLimitErr,
	}

	backend := s.defaultBackend(query_mode.IndexQueryModeFailover)

	for _, errToReturn := range errors {
		s.Run(fmt.Sprintf("ExecuteScriptAtLatestBlock - recovers %v", errToReturn), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
				Return(nil, errToReturn).Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testExecuteScriptAtLatestBlock(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockID - recovers %v", errToReturn), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
				Return(nil, errToReturn).Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testExecuteScriptAtBlockID(ctx, backend, codes.OK)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockHeight - recovers %v", errToReturn), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.script, s.arguments, s.block.Height, s.registers).
				Return(nil, errToReturn).Once()
			s.setupENSuccessResponse(s.block.ID())

			s.testExecuteScriptAtBlockHeight(ctx, backend, codes.OK)
		})
	}

	s.executionResultProvider.AssertExpectations(s.T())
	s.executionStateCache.AssertExpectations(s.T())
	s.executionDataSnapshot.AssertExpectations(s.T())
	s.scriptExecutor.AssertExpectations(s.T())
	s.connectionFactory.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}

// TestExecuteScriptWithFailover_SkippedForCorrectCodes tests that failover is skipped for
// FVM errors that result in InvalidArgument or Canceled errors.
func (s *BackendScriptsSuite) TestExecuteScriptWithFailover_SkippedForCorrectCodes() {
	ctx := context.Background()
	backend := s.defaultBackend(query_mode.IndexQueryModeFailover)

	testCases := []struct {
		err        error
		statusCode codes.Code
	}{
		{
			err:        cadenceErr,
			statusCode: codes.InvalidArgument,
		},
		{
			err:        ctxCancelErr,
			statusCode: codes.Canceled,
		},
	}

	for _, tt := range testCases {
		s.Run(fmt.Sprintf("ExecuteScriptAtLatestBlock - %s", tt.statusCode), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()

			s.testExecuteScriptAtLatestBlock(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockID - %s", tt.statusCode), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()

			s.testExecuteScriptAtBlockID(ctx, backend, tt.statusCode)
		})

		s.Run(fmt.Sprintf("ExecuteScriptAtBlockHeight - %s", tt.statusCode), func() {
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
			s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, s.failingScript, s.arguments, s.block.Height, s.registers).
				Return(nil, tt.err).
				Once()

			s.testExecuteScriptAtBlockHeight(ctx, backend, tt.statusCode)
		})
	}

	s.executionResultProvider.AssertExpectations(s.T())
	s.executionStateCache.AssertExpectations(s.T())
	s.executionDataSnapshot.AssertExpectations(s.T())
	s.scriptExecutor.AssertExpectations(s.T())
}

// TestExecuteScriptWithFailover_ReturnsENErrors tests that when an error is returned from the execution
// node during a failover, it is returned to the caller.
func (s *BackendScriptsSuite) TestExecuteScriptWithFailover_ReturnsENErrors() {
	ctx := context.Background()

	// use a status code that's not used in the API to make sure it's passed through
	statusCode := codes.FailedPrecondition
	errToReturn := status.Error(statusCode, "random error")

	backend := s.defaultBackend(query_mode.IndexQueryModeFailover)

	s.Run("ExecuteScriptAtLatestBlock", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtLatestBlock(ctx, backend, statusCode)
	})

	s.Run("ExecuteScriptAtBlockID", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtBlockID(ctx, backend, statusCode)
	})

	s.Run("ExecuteScriptAtBlockHeight", func() {
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
		s.scriptExecutor.On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, s.block.Height, s.registers).
			Return(nil, storage.ErrHeightNotIndexed).
			Once()
		s.setupENFailingResponse(s.block.ID(), errToReturn)

		s.testExecuteScriptAtBlockHeight(ctx, backend, statusCode)
	})

	s.executionResultProvider.AssertExpectations(s.T())
	s.executionStateCache.AssertExpectations(s.T())
	s.executionDataSnapshot.AssertExpectations(s.T())
	s.scriptExecutor.AssertExpectations(s.T())
	s.connectionFactory.AssertExpectations(s.T())
	s.execClient.AssertExpectations(s.T())
}

// TestExecuteScriptAtLatestBlockFromStorage_InconsistentState tests that signaler context received error when node state is
// inconsistent.
func (s *BackendScriptsSuite) TestExecuteScriptAtLatestBlockFromStorage_InconsistentState() {
	scripts := s.defaultBackend(query_mode.IndexQueryModeLocalOnly)

	s.state.On("Sealed").Return(s.snapshot, nil).Once()

	err := fmt.Errorf("inconsistent node state")
	s.snapshot.On("Head").Return(nil, err).Once()

	signCtxErr := irrecoverable.NewExceptionf("failed to lookup latest sealed header: %w", err)
	signalerCtx := irrecoverable.WithSignalerContext(context.Background(),
		irrecoverable.NewMockSignalerContextExpectError(s.T(), context.Background(), signCtxErr))

	actual, metadata, err := scripts.ExecuteScriptAtLatestBlock(signalerCtx, s.script, s.arguments, s.criteria)
	s.Require().Error(err)
	s.Require().Nil(actual)
	s.Require().Nil(metadata)

	s.state.AssertExpectations(s.T())
	s.snapshot.AssertExpectations(s.T())
}

// TestExecuteScript_ExceedsMaxSize tests that when a script exceeds the max size, it returns an error.
func (s *BackendScriptsSuite) TestExecuteScript_ExceedsMaxSize() {
	ctx := context.Background()
	script := unittest.RandomBytes(commonrpc.DefaultAccessMaxRequestSize + 1)
	scripts := s.defaultBackend(query_mode.IndexQueryModeLocalOnly)

	s.Run("ExecuteScriptAtLatestBlock", func() {
		actual, metadata, err := scripts.ExecuteScriptAtLatestBlock(ctx, script, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().True(access.IsInvalidRequestError(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	})

	s.Run("ExecuteScriptAtBlockID", func() {
		actual, metadata, err := scripts.ExecuteScriptAtBlockID(ctx, s.block.ID(), script, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().True(access.IsInvalidRequestError(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	})

	s.Run("ExecuteScriptAtBlockHeight", func() {
		actual, metadata, err := scripts.ExecuteScriptAtBlockHeight(ctx, s.block.Height, script, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().True(access.IsInvalidRequestError(err))
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	})
}

// testExecuteScriptAtLatestBlock tests the Scripts.ExecuteScriptAtLatestBlock method.
//
// It verifies that the method correctly executes a script against the latest sealed block
// and returns the expected response or error. The test behavior depends on the provided
// expected gRPC status code:
//   - If expectedStatusCode == codes.OK, it asserts that the script executes successfully
//     and returns the expected response.
//   - Otherwise, it asserts that an error occurs with the expected status code,
//     and both the actual result and metadata are nil.
func (s *BackendScriptsSuite) testExecuteScriptAtLatestBlock(
	ctx context.Context,
	scripts *Scripts,
	expectedStatusCode codes.Code,
) {
	s.state.On("Sealed").Return(s.snapshot, nil).Once()
	s.snapshot.On("Head").Return(s.block.ToHeader(), nil).Once()

	if expectedStatusCode == codes.OK {
		actual, metadata, err := scripts.ExecuteScriptAtLatestBlock(ctx, s.script, s.arguments, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(expectedResponse, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := scripts.ExecuteScriptAtLatestBlock(ctx, s.failingScript, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().Equal(expectedStatusCode, status.Code(err), "error code mismatch: expected %d, got %d: %s", expectedStatusCode, status.Code(err), err)
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}

	s.state.AssertExpectations(s.T())
	s.snapshot.AssertExpectations(s.T())
}

// testExecuteScriptAtBlockID tests the Scripts.ExecuteScriptAtBlockID method.
//
// It verifies that the method correctly executes a script at a specific block ID
// and produces the expected result or error. The test behavior depends on the
// expected gRPC status code:
//   - If expectedStatusCode == codes.OK, the script should execute successfully
//     and return the expected response.
//   - Otherwise, it asserts that an error occurs with the expected status code,
//     and both the actual result and metadata are nil.
func (s *BackendScriptsSuite) testExecuteScriptAtBlockID(
	ctx context.Context,
	scripts *Scripts,
	expectedStatusCode codes.Code,
) {
	blockID := s.block.ID()
	s.headers.On("ByBlockID", blockID).Return(s.block.ToHeader(), nil).Once()

	if expectedStatusCode == codes.OK {
		actual, metadata, err := scripts.ExecuteScriptAtBlockID(ctx, blockID, s.script, s.arguments, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(expectedResponse, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := scripts.ExecuteScriptAtBlockID(ctx, blockID, s.failingScript, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().Equal(expectedStatusCode, status.Code(err), "error code mismatch: expected %d, got %d: %s", expectedStatusCode, status.Code(err), err)
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}

	s.headers.AssertExpectations(s.T())
}

// testExecuteScriptAtBlockHeight tests the Scripts.ExecuteScriptAtBlockHeight method.
//
// It verifies that the method correctly executes a script against a specific block height
// and returns the expected output or error. The test behavior is determined by the
// expected gRPC status code:
//   - If expectedStatusCode == codes.OK, the execution is expected to succeed
//     and produce the expected response.
//   - Otherwise, it verifies that an error occurs with the expected status code,
//     and both the actual result and metadata are nil.
func (s *BackendScriptsSuite) testExecuteScriptAtBlockHeight(
	ctx context.Context,
	scripts *Scripts,
	expectedStatusCode codes.Code,
) {
	height := s.block.Height
	s.headers.On("ByHeight", height).Return(s.block.ToHeader(), nil).Once()

	if expectedStatusCode == codes.OK {
		actual, metadata, err := scripts.ExecuteScriptAtBlockHeight(ctx, height, s.script, s.arguments, s.criteria)
		s.Require().NoError(err)
		s.Require().Equal(expectedResponse, actual)
		s.Require().Equal(s.expectedMetadata, metadata)
	} else {
		actual, metadata, err := scripts.ExecuteScriptAtBlockHeight(ctx, height, s.failingScript, s.arguments, s.criteria)
		s.Require().Error(err)
		s.Require().Equalf(expectedStatusCode, status.Code(err), "error code mismatch: expected %d, got %d: %s", expectedStatusCode, status.Code(err), err)
		s.Require().Nil(actual)
		s.Require().Nil(metadata)
	}

	s.headers.AssertExpectations(s.T())
}
