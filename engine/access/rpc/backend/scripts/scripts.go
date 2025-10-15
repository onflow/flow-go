package scripts

import (
	"context"
	"crypto/md5" //nolint:gosec
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

	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
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
	cache := executor.NewLoggedScriptCache(log, loggedScripts)

	switch scriptExecMode {
	case query_mode.IndexQueryModeLocalOnly:
		exec = executor.NewLocalScriptExecutor(
			log,
			metrics,
			scriptExecutor,
			cache,
			executionStateCache,
		)

	case query_mode.IndexQueryModeExecutionNodesOnly:
		exec = executor.NewENScriptExecutor(
			log,
			metrics,
			nodeProvider,
			nodeCommunicator,
			connFactory,
			cache,
		)

	case query_mode.IndexQueryModeFailover:
		local := executor.NewLocalScriptExecutor(
			log,
			metrics,
			scriptExecutor,
			cache,
			executionStateCache,
		)
		execNode := executor.NewENScriptExecutor(
			log,
			metrics,
			nodeProvider,
			nodeCommunicator,
			connFactory,
			cache,
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
		executionStateCache:      executionStateCache,
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
// Expected errors:
//   - [access.InvalidRequestError] - the combined size (in bytes) of the script and arguments is greater than the max size
//   - [access.DataNotFoundError] - when data required to process the request is not available.
func (b *Scripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, access.NewInvalidRequestError(commonrpc.ErrScriptTooLarge)
	}

	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		err = irrecoverable.NewExceptionf("failed to lookup latest sealed header: %w", err)
		return nil, nil, access.RequireNoError(ctx, err)
	}

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(
		latestHeader.ID(),
		userCriteria,
	)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		}
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("execution data", err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(latestHeader.ID(), latestHeader.Height, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}

// ExecuteScriptAtBlockID executes provided script at the provided block ID.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected errors:
//   - [access.InvalidRequestError] - the combined size (in bytes) of the script and arguments is greater than the max size.
//   - [access.DataNotFoundError] - when data required to process the request is not available.
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
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		}
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("execution data", err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(blockID, header.Height, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}

// ExecuteScriptAtBlockHeight executes provided script at the provided block height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
// As documented in the [access.API], which we partially implement with this function
//   - All errors returned by this API are guaranteed to be benign. The node can continue normal operations after such errors.
//   - Hence, we MUST check here and crash on all errors *except* for those known to be benign in the present context!
//
// Expected errors:
//   - [access.InvalidRequestError] - the combined size (in bytes) of the script and arguments is greater than the max size.
//   - [access.DataNotFoundError] - when data required to process the request is not available.
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

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(header.ID(), userCriteria)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		if common.IsInsufficientExecutionReceipts(err) {
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		}
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		return nil, nil, access.NewDataNotFoundError("execution data", err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(header.ID(), blockHeight, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}
