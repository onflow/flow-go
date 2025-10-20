package executor

import (
	"context"
	//nolint:gosec
	"time"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"

	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// TODO(Uliana): add godoc to whole file
type ENScriptExecutor struct {
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics //TODO: move this metrics to scriptLogger struct?

	nodeProvider     *commonrpc.ExecutionNodeIdentitiesProvider
	nodeCommunicator node_communicator.Communicator
	connFactory      connection.ConnectionFactory

	scriptLogger *ScriptLogger
}

var _ ScriptExecutor = (*ENScriptExecutor)(nil)

func NewENScriptExecutor(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	nodeProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	nodeCommunicator node_communicator.Communicator,
	connFactory connection.ConnectionFactory,
	scriptLogger *ScriptLogger,
) *ENScriptExecutor {
	return &ENScriptExecutor{
		log:              zerolog.New(log).With().Str("script_executor", "execution_node").Logger(),
		metrics:          metrics,
		nodeProvider:     nodeProvider,
		nodeCommunicator: nodeCommunicator,
		connFactory:      connFactory,
		scriptLogger:     scriptLogger,
	}
}

func (e *ENScriptExecutor) Execute(ctx context.Context, request *Request, executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]byte, *accessmodel.ExecutorMetadata, error) {
	var result []byte
	var executionTime time.Time
	var execDuration time.Duration
	var nodeID flow.Identifier
	var err error
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		executionResultInfo.ExecutionNodes,
		func(node *flow.IdentitySkeleton) error {
			execStartTime := time.Now()

			result, err = e.tryExecuteScriptOnExecutionNode(ctx, node.Address, request)

			executionTime = time.Now()
			execDuration = executionTime.Sub(execStartTime)
			nodeID = node.NodeID

			if err != nil {
				return err
			}

			e.scriptLogger.LogExecutedScript(request, node.Address, execDuration)
			e.metrics.ScriptExecuted(time.Since(execStartTime), len(request.script))

			return nil
		},
		func(node *flow.IdentitySkeleton, err error) bool {
			if status.Code(err) == codes.InvalidArgument {
				e.scriptLogger.LogFailedScript(request, node.Address)
				return true
			}
			return false
		},
	)

	if errToReturn != nil {
		if status.Code(errToReturn) != codes.InvalidArgument {
			e.metrics.ScriptExecutionErrorOnExecutionNode()
			e.log.Error().Err(errToReturn).Msg("script execution failed for execution node internal reasons")
		}
		return nil, nil, rpc.ConvertError(errToReturn, "failed to execute script on execution nodes", codes.Internal)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       common.OrderedExecutors(nodeID, executionResultInfo.ExecutionNodes.NodeIDs()),
	}

	return result, metadata, nil
}

// tryExecuteScriptOnExecutionNode attempts to execute the script on the given execution node.
func (e *ENScriptExecutor) tryExecuteScriptOnExecutionNode(
	ctx context.Context,
	executorAddress string,
	r *Request,
) ([]byte, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create client for execution node %s: %v", executorAddress, err)
	}
	defer closer.Close()

	execResp, err := execRPCClient.ExecuteScriptAtBlockID(
		ctx, &execproto.ExecuteScriptAtBlockIDRequest{
			BlockId:   r.blockID[:],
			Script:    r.script,
			Arguments: r.arguments,
		},
	)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "failed to execute the script on the execution node %s: %v", executorAddress, err)
	}

	return execResp.GetValue(), nil
}
