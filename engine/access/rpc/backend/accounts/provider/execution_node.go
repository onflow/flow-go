package provider

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/backend/common"
	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/state/protocol"
)

type ENAccountProvider struct {
	log                        zerolog.Logger
	state                      protocol.State
	connFactory                connection.ConnectionFactory
	nodeCommunicator           node_communicator.Communicator
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
}

var _ AccountProvider = (*ENAccountProvider)(nil)

func NewENAccountProvider(
	log zerolog.Logger,
	state protocol.State,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentityProvider *rpc.ExecutionNodeIdentitiesProvider,
) *ENAccountProvider {
	return &ENAccountProvider{
		log:                        log.With().Str("account_provider", "execution_node").Logger(),
		state:                      state,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		execNodeIdentitiesProvider: execNodeIdentityProvider,
	}
}

func (e *ENAccountProvider) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	_ uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.Account, *accessmodel.ExecutorMetadata, error) {
	req := &execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	var resp *execproto.GetAccountAtBlockIDResponse
	var nodeID flow.Identifier
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		executionResultInfo.ExecutionNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			start := time.Now()

			resp, err = e.tryGetAccount(ctx, node.Address, req)
			nodeID = node.NodeID
			duration := time.Since(start)

			lg := e.log.With().
				Str("execution_node", node.String()).
				Hex("block_id", req.GetBlockId()).
				Hex("address", req.GetAddress()).
				Int64("rtt_ms", duration.Milliseconds()).
				Logger()

			if err != nil {
				lg.Err(err).Msg("failed to execute GetAccount")
				return err
			}

			// return if any execution node replied successfully
			lg.Debug().Msg("Successfully got account info")
			return nil
		},
		nil,
	)

	if errToReturn != nil {
		return nil, nil, rpc.ConvertError(errToReturn, "failed to get account from the execution node", codes.Internal)
	}

	account, err := convert.MessageToAccount(resp.GetAccount())
	if err != nil {
		return nil, nil, status.Errorf(codes.Internal, "failed to convert account message: %v", err)
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultInfo.ExecutionResultID,
		ExecutorIDs:       common.OrderedExecutors(nodeID, executionResultInfo.ExecutionNodes.NodeIDs()),
	}

	return account, metadata, nil
}

func (e *ENAccountProvider) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (uint64, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return 0, nil, err
	}

	return account.Balance, metadata, nil
}

func (e *ENAccountProvider) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) (*flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return nil, nil, err
	}

	for _, key := range account.Keys {
		if key.Index == keyIndex {
			return &key, metadata, nil
		}
	}

	return nil, nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)
}

func (e *ENAccountProvider) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
	executionResultInfo *optimistic_sync.ExecutionResultInfo,
) ([]flow.AccountPublicKey, *accessmodel.ExecutorMetadata, error) {
	account, metadata, err := e.GetAccountAtBlock(ctx, address, blockID, height, executionResultInfo)
	if err != nil {
		return nil, nil, err
	}

	return account.Keys, metadata, nil
}

// tryGetAccount attempts to get the account from the given execution node.
func (e *ENAccountProvider) tryGetAccount(
	ctx context.Context,
	executorAddress string,
	req *execproto.GetAccountAtBlockIDRequest,
) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(executorAddress)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "failed to create client for execution node %s: %v", executorAddress, err)
	}
	defer closer.Close()

	return execRPCClient.GetAccountAtBlockID(ctx, req)
}
