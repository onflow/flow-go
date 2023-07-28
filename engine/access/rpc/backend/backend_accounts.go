package backend

import (
	"context"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendAccounts struct {
	state             protocol.State
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	connFactory       connection.ConnectionFactory
	log               zerolog.Logger
	nodeCommunicator  *NodeCommunicator
}

func (b *backendAccounts) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return b.GetAccountAtLatestBlock(ctx, address)
}

func (b *backendAccounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {

	// get the latest sealed header
	latestHeader, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	latestBlockID := latestHeader.ID()

	account, err := b.getAccountAtBlockID(ctx, address, latestBlockID)
	if err != nil {
		b.log.Error().Err(err).Msgf("failed to get account at blockID: %v", latestBlockID)
		return nil, err
	}

	return account, nil
}

func (b *backendAccounts) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (*flow.Account, error) {
	// get header at given height
	header, err := b.headers.ByHeight(height)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	// get block ID of the header at the given height
	blockID := header.ID()

	account, err := b.getAccountAtBlockID(ctx, address, blockID)
	if err != nil {
		return nil, err
	}

	return account, nil
}

func (b *backendAccounts) getAccountAtBlockID(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
) (*flow.Account, error) {

	exeReq := &execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to get account from the execution node", codes.Internal)
	}

	var exeRes *execproto.GetAccountAtBlockIDResponse
	exeRes, err = b.getAccountFromAnyExeNode(ctx, execNodes, exeReq)
	if err != nil {
		b.log.Error().Err(err).Msg("failed to get account from execution nodes")
		return nil, rpc.ConvertError(err, "failed to get account from the execution node", codes.Internal)
	}

	account, err := convert.MessageToAccount(exeRes.GetAccount())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert account message: %v", err)
	}

	return account, nil
}

// getAccountFromAnyExeNode retrieves the given account from any EN in `execNodes`.
// We attempt querying each EN in sequence. If any EN returns a valid response, then errors from
// other ENs are logged and swallowed. If all ENs fail to return a valid response, then an
// error aggregating all failures is returned.
func (b *backendAccounts) getAccountFromAnyExeNode(ctx context.Context, execNodes flow.IdentityList, req *execproto.GetAccountAtBlockIDRequest) (*execproto.GetAccountAtBlockIDResponse, error) {
	var resp *execproto.GetAccountAtBlockIDResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			// TODO: use the GRPC Client interceptor
			start := time.Now()

			resp, err = b.tryGetAccount(ctx, node, req)
			duration := time.Since(start)
			if err == nil {
				// return if any execution node replied successfully
				b.log.Debug().
					Str("execution_node", node.String()).
					Hex("block_id", req.GetBlockId()).
					Hex("address", req.GetAddress()).
					Int64("rtt_ms", duration.Milliseconds()).
					Msg("Successfully got account info")
				return nil
			}
			b.log.Error().
				Str("execution_node", node.String()).
				Hex("block_id", req.GetBlockId()).
				Hex("address", req.GetAddress()).
				Int64("rtt_ms", duration.Milliseconds()).
				Err(err).
				Msg("failed to execute GetAccount")
			return err
		},
		nil,
	)

	return resp, errToReturn
}

func (b *backendAccounts) tryGetAccount(ctx context.Context, execNode *flow.Identity, req *execproto.GetAccountAtBlockIDRequest) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	resp, err := execRPCClient.GetAccountAtBlockID(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
