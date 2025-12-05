package scripts

import (
	"context"
	"crypto/md5" //nolint:gosec
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/backend/query_mode"
	"github.com/onflow/flow-go/engine/access/rpc/backend/scripts/executor"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Scripts implements the script-related part of the Access API.
//
// It provides methods to execute Cadence scripts against the Flow blockchain state
// at a specific block height, block ID, or the latest sealed block.
//
// The execution strategy depends on the configured query mode and can run scripts
// locally, on remote execution nodes, or in a failover mode combining both.
type Scripts struct {
	headers                  storage.Headers
	state                    protocol.State
	executor                 executor.ScriptExecutor
	maxScriptAndArgumentSize uint
	executionResultProvider  optimistic_sync.ExecutionResultInfoProvider
}

var _ access.ScriptsAPI = (*Scripts)(nil)

// NewScriptsBackend constructs and initializes a new Scripts backend instance.
//
// No errors are expected during normal operation.
func NewScriptsBackend(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	headers storage.Headers,
	state protocol.State,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	scriptExecutor execution.ScriptExecutor,
	scriptExecMode query_mode.IndexQueryMode,
	nodeProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	loggedScripts *lru.Cache[[md5.Size]byte, time.Time],
	maxScriptAndArgumentSize uint,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
) (*Scripts, error) {
	var exec executor.ScriptExecutor
	scriptLogger := executor.NewScriptLogger(log, loggedScripts)

	switch scriptExecMode {
	case query_mode.IndexQueryModeLocalOnly:
		exec = executor.NewLocalScriptExecutor(
			log,
			metrics,
			scriptExecutor,
			scriptLogger,
			executionStateCache,
		)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		exec = executor.NewENScriptExecutor(
			log,
			metrics,
			nodeProvider,
			nodeCommunicator,
			connFactory,
			scriptLogger,
		)

	case query_mode.IndexQueryModeFailover:
		local := executor.NewLocalScriptExecutor(
			log,
			metrics,
			scriptExecutor,
			scriptLogger,
			executionStateCache,
		)
		execNode := executor.NewENScriptExecutor(
			log,
			metrics,
			nodeProvider,
			nodeCommunicator,
			connFactory,
			scriptLogger,
		)
		exec = executor.NewFailoverScriptExecutor(local, execNode)

	default:
		return nil, fmt.Errorf("invalid index mode: %s", scriptExecMode.String())
	}

	return &Scripts{
		headers:                  headers,
		state:                    state,
		executor:                 exec,
		maxScriptAndArgumentSize: maxScriptAndArgumentSize,
		executionResultProvider:  executionResultProvider,
	}, nil
}

// ExecuteScriptAtLatestBlock executes provided script at the latest sealed block.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: The combined size (in bytes) of the script and arguments is greater than the max size or
//     if the script execution failed due to invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node could not be established.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (b *Scripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, access.NewInvalidRequestError(commonrpc.ErrScriptTooLarge)
	}

	sealedHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		err = irrecoverable.NewExceptionf("failed to lookup latest sealed header: %w", err)
		return nil, nil, access.RequireNoError(ctx, err)
	}

	sealedHeaderID := sealedHeader.ID()
	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(
		sealedHeaderID,
		userCriteria,
	)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsInsufficientExecutionReceipts(err):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsRequiredExecutorsCountExceeded(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsUnknownRequiredExecutor(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsCriteriaNotMetError(err):
			return nil, nil, access.NewPreconditionFailedError(err)
		default:
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	request := executor.NewScriptExecutionRequest(sealedHeaderID, sealedHeader.Height, script, arguments)
	res, metadata, err := b.executor.Execute(ctx, request, executionResultInfo)
	if err != nil {
		return nil, nil, access.RequireAccessError(ctx, err)
	}

	return res, metadata, nil
}

// ExecuteScriptAtBlockID executes provided script at the provided block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: The combined size (in bytes) of the script and arguments is greater than the max size or
//     if the script execution failed due to invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node could not be established.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (b *Scripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, access.NewInvalidRequestError(commonrpc.ErrScriptTooLarge)
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find header by ID: %w", err)
		return nil, nil, access.NewDataNotFoundError("header", err)
	}

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(blockID, userCriteria)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsInsufficientExecutionReceipts(err):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsRequiredExecutorsCountExceeded(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsUnknownRequiredExecutor(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsCriteriaNotMetError(err):
			return nil, nil, access.NewPreconditionFailedError(err)
		default:
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	res, metadata, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(blockID, header.Height, script, arguments),
		executionResultInfo,
	)
	if err != nil {
		return nil, nil, access.RequireAccessError(ctx, err)
	}

	return res, metadata, nil
}

// ExecuteScriptAtBlockHeight executes provided script at the provided block height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected error returns during normal operation:
//   - [access.InvalidRequestError]: The combined size (in bytes) of the script and arguments is greater than the max size or
//     if the script execution failed due to invalid arguments.
//   - [access.ResourceExhausted]: If computation or memory limits were exceeded.
//   - [access.DataNotFoundError]: If data required to process the request is not available.
//   - [access.OutOfRangeError]: If the requested data is outside the available range.
//   - [access.PreconditionFailedError]: If data for block is not available.
//   - [access.RequestCanceledError]: If the script execution was canceled.
//   - [access.RequestTimedOutError]: If the script execution timed out.
//   - [access.ServiceUnavailable]: If no nodes are available or a connection to an execution node could not be established.
//   - [access.InternalError]: For internal failures or index conversion errors.
func (b *Scripts) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, access.NewInvalidRequestError(commonrpc.ErrScriptTooLarge)
	}

	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = common.ResolveHeightError(b.state.Params(), blockHeight, err)
		err = fmt.Errorf("failed to find header by height: %w", err)
		return nil, nil, access.NewDataNotFoundError("header", err)
	}

	headerID := header.ID()
	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(headerID, userCriteria)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsInsufficientExecutionReceipts(err):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case common.IsRequiredExecutorsCountExceeded(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsUnknownRequiredExecutor(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case common.IsCriteriaNotMetError(err):
			return nil, nil, access.NewPreconditionFailedError(err)
		default:
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	res, metadata, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(headerID, blockHeight, script, arguments),
		executionResultInfo,
	)
	if err != nil {
		return nil, nil, access.RequireAccessError(ctx, err)
	}

	return res, metadata, nil
}
