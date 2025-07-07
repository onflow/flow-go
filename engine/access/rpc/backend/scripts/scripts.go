package scripts

import (
	"context"
	"crypto/md5"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/access/rpc/backend/scripts/executor"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Scripts struct {
	headers  storage.Headers
	state    protocol.State
	executor executor.ScriptExecutor
}

func NewScripts(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	headers storage.Headers,
	state protocol.State,
	connFactory connection.ConnectionFactory,
	nodeCommunicator backend.Communicator,
	scriptExecutor execution.ScriptExecutor,
	scriptExecMode backend.IndexQueryMode,
	nodeProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	loggedScripts *lru.Cache[[md5.Size]byte, time.Time],
) (*Scripts, error) {
	var exec executor.ScriptExecutor
	cache := executor.NewScriptCache(log, loggedScripts)

	switch scriptExecMode {
	case backend.IndexQueryModeLocalOnly:
		exec = executor.NewLocalExecutor(log, metrics, scriptExecutor, cache)

	case backend.IndexQueryModeExecutionNodesOnly:
		exec = executor.NewExecutionNodeExecutor(log, metrics, nodeProvider, nodeCommunicator, connFactory, cache)

	case backend.IndexQueryModeFailover:
		local := executor.NewLocalExecutor(log, metrics, scriptExecutor, cache)
		execNode := executor.NewExecutionNodeExecutor(log, metrics, nodeProvider, nodeCommunicator, connFactory, cache)
		exec = executor.NewFailoverExecutor(local, execNode)

	case backend.IndexQueryModeCompare:
		local := executor.NewLocalExecutor(log, metrics, scriptExecutor, cache)
		execNode := executor.NewExecutionNodeExecutor(log, metrics, nodeProvider, nodeCommunicator, connFactory, cache)
		exec = executor.NewCompareExecutor(log, metrics, cache, local, execNode)

	default:
		return nil, status.Error(codes.InvalidArgument, "invalid index mode")
	}

	return &Scripts{
		headers:  headers,
		state:    state,
		executor: exec,
	}, nil
}

// ExecuteScriptAtLatestBlock executes provided script at the latest sealed block.
func (b *Scripts) ExecuteScriptAtLatestBlock(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		// the latest sealed header MUST be available
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	res, _, err := b.executor.Execute(ctx, executor.NewScriptExecutionRequest(latestHeader.ID(), latestHeader.Height, script, arguments))
	return res, err
}

// ExecuteScriptAtBlockID executes provided script at the provided block ID.
func (b *Scripts) ExecuteScriptAtBlockID(
	ctx context.Context,
	blockID flow.Identifier,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	header, err := b.headers.ByBlockID(blockID)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(err)
	}

	res, _, err := b.executor.Execute(ctx, executor.NewScriptExecutionRequest(blockID, header.Height, script, arguments))
	return res, err
}

// ExecuteScriptAtBlockHeight executes provided script at the provided block height.
func (b *Scripts) ExecuteScriptAtBlockHeight(
	ctx context.Context,
	blockHeight uint64,
	script []byte,
	arguments [][]byte,
) ([]byte, error) {
	header, err := b.headers.ByHeight(blockHeight)
	if err != nil {
		return nil, commonrpc.ConvertStorageError(backend.ResolveHeightError(b.state.Params(), blockHeight, err))
	}

	res, _, err := b.executor.Execute(ctx, executor.NewScriptExecutionRequest(header.ID(), blockHeight, script, arguments))
	return res, err
}
