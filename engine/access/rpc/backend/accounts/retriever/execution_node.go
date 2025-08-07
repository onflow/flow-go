package retriever

import (
	"context"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/backend/node_communicator"
	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type ENAccountRetriever struct {
	log                        zerolog.Logger
	state                      protocol.State
	connFactory                connection.ConnectionFactory
	nodeCommunicator           node_communicator.Communicator
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
}

var _ AccountRetriever = (*ENAccountRetriever)(nil)

func NewENAccountRetriever(
	log zerolog.Logger,
	state protocol.State,
	connFactory connection.ConnectionFactory,
	nodeCommunicator node_communicator.Communicator,
	execNodeIdentityProvider *rpc.ExecutionNodeIdentitiesProvider,
) *ENAccountRetriever {
	return &ENAccountRetriever{
		log:                        log.With().Str("account_retriever", "execution_node").Logger(),
		state:                      state,
		connFactory:                connFactory,
		nodeCommunicator:           nodeCommunicator,
		execNodeIdentitiesProvider: execNodeIdentityProvider,
	}
}

func (e *ENAccountRetriever) GetAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	_ uint64,
) (*flow.Account, error) {
	req := &execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	execNodes, err := e.execNodeIdentitiesProvider.ExecutionNodesForBlockID(
		ctx,
		blockID,
	)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to find execution node to query", codes.Internal)
	}

	var resp *execproto.GetAccountAtBlockIDResponse
	errToReturn := e.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
			var err error
			start := time.Now()

			resp, err = e.tryGetAccount(ctx, node, req)
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
		return nil, rpc.ConvertError(errToReturn, "failed to get account from the execution node", codes.Internal)
	}

	account, err := convert.MessageToAccount(resp.GetAccount())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert account message: %v", err)
	}

	return account, nil
}

func (e *ENAccountRetriever) GetAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	account, err := e.GetAccountAtBlock(ctx, address, blockID, height)
	if err != nil {
		return 0, err
	}

	return account.Balance, nil
}

func (e *ENAccountRetriever) GetAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	account, err := e.GetAccountAtBlock(ctx, address, blockID, height)
	if err != nil {
		return nil, err
	}

	for _, key := range account.Keys {
		if key.Index == keyIndex {
			return &key, nil
		}
	}

	return nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)
}

func (e *ENAccountRetriever) GetAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	account, err := e.GetAccountAtBlock(ctx, address, blockID, height)
	if err != nil {
		return nil, err
	}

	return account.Keys, nil
}

// tryGetAccount attempts to get the account from the given execution node.
func (e *ENAccountRetriever) tryGetAccount(
	ctx context.Context,
	execNode *flow.IdentitySkeleton,
	req *execproto.GetAccountAtBlockIDRequest,
) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := e.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return execRPCClient.GetAccountAtBlockID(ctx, req)
}
