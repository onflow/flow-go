package scripts

import (
	"context"
	"crypto/md5" //nolint:gosec
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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

// TODO(Uliana): add godoc to whole file
type Scripts struct {
	headers                  storage.Headers
	state                    protocol.State
	executor                 executor.ScriptExecutor
	maxScriptAndArgumentSize uint

	executionResultProvider optimistic_sync.ExecutionResultInfoProvider
	executionStateCache     optimistic_sync.ExecutionStateCache
}

var _ access.ScriptsAPI = (*Scripts)(nil)

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
func (b *Scripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, status.Error(codes.InvalidArgument, commonrpc.ErrScriptTooLarge.Error())
	}

	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, nil, err
	}

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(
		latestHeader.ID(),
		userCriteria,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result for block ID %s: %w", latestHeader.ID(), err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(latestHeader.ID(), latestHeader.Height, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}

// ExecuteScriptAtBlockID executes provided script at the provided block ID.
func (b *Scripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, status.Error(codes.InvalidArgument, commonrpc.ErrScriptTooLarge.Error())
	}

	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, nil, commonrpc.ConvertStorageError(err)
	}

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(blockID, userCriteria)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result for block ID %s: %w", blockID.String(), err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(blockID, header.Height, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}

// ExecuteScriptAtBlockHeight executes provided script at the provided block height.
func (b *Scripts) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
	userCriteria optimistic_sync.Criteria,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	if !commonrpc.CheckScriptSize(script, arguments, b.maxScriptAndArgumentSize) {
		return nil, nil, status.Error(codes.InvalidArgument, commonrpc.ErrScriptTooLarge.Error())
	}

	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		return nil, nil, commonrpc.ConvertStorageError(common.ResolveHeightError(b.state.Params(), blockHeight, err))
	}

	executionResultInfo, err := b.executionResultProvider.ExecutionResultInfo(header.ID(), userCriteria)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get execution result for block ID %s: %w", header.ID(), err)
	}

	res, metadata, _, err := b.executor.Execute(
		ctx,
		executor.NewScriptExecutionRequest(header.ID(), blockHeight, script, arguments, executionResultInfo),
	)
	return res, metadata, err
}
