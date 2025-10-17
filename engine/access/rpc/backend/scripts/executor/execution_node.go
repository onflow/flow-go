package executor

import (
	"context"
	"crypto/md5" //nolint:gosec
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
)

// TODO(Uliana): add godoc to whole file
type ENScriptExecutor struct {
	log     zerolog.Logger
	metrics module.BackendScriptsMetrics //TODO: move this metrics to scriptCache struct?

	nodeProvider     *commonrpc.ExecutionNodeIdentitiesProvider
	nodeCommunicator node_communicator.Communicator
	connFactory      connection.ConnectionFactory

	scriptCache *LoggedScriptCache
}

var _ ScriptExecutor = (*ENScriptExecutor)(nil)

func NewENScriptExecutor(
	log zerolog.Logger,
	metrics module.BackendScriptsMetrics,
	nodeProvider *commonrpc.ExecutionNodeIdentitiesProvider,
	nodeCommunicator node_communicator.Communicator,
	connFactory connection.ConnectionFactory,
	scriptCache *LoggedScriptCache,
) *ENScriptExecutor {
	return &ENScriptExecutor{
		log:              zerolog.New(log).With().Str("script_executor", "execution_node").Logger(),
		metrics:          metrics,
		nodeProvider:     nodeProvider,
		nodeCommunicator: nodeCommunicator,
		connFactory:      connFactory,
		scriptCache:      scriptCache,
	}
}

func (e *ENScriptExecutor) Execute(ctx context.Context, request *Request) ([]byte, *accessmodel.ExecutorMetadata, error) {
	// encode to MD5 as low compute/memory lookup key
	// CAUTION: cryptographically insecure md5 is used here, but only to de-duplicate logs.
	// *DO NOT* use this hash for any protocol-related or cryptographic functions.
	insecureScriptHash := md5.Sum(request.script) //nolint:gosec

	var result []byte
	var executionTime time.Time
	var execDuration time.Duration
	var nodeID flow.Identifier
	var err error
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		request.execResultInfo.ExecutionNodes,
		func(node *flow.IdentitySkeleton) error {
			execStartTime := time.Now()

			result, err = e.tryExecuteScriptOnExecutionNode(ctx, node.Address, request)

			executionTime = time.Now()
			execDuration = executionTime.Sub(execStartTime)
			nodeID = node.NodeID

			if err != nil {
				return err
			}

			e.scriptCache.LogExecutedScript(request.blockID, insecureScriptHash, executionTime, node.Address, request.script, execDuration)
			e.metrics.ScriptExecuted(time.Since(execStartTime), len(request.script))

			return nil
		},
		func(node *flow.IdentitySkeleton, err error) bool {
			if status.Code(err) == codes.InvalidArgument {
				e.scriptCache.LogFailedScript(request.blockID, insecureScriptHash, executionTime, node.Address, request.script)
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
		ExecutionResultID: request.execResultInfo.ExecutionResultID,
		ExecutorIDs:       common.OrderedExecutors(nodeID, request.execResultInfo.ExecutionNodes.NodeIDs()),
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
