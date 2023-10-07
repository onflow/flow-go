package backend

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type backendAccounts struct {
	log               zerolog.Logger
	state             protocol.State
	headers           storage.Headers
	executionReceipts storage.ExecutionReceipts
	connFactory       connection.ConnectionFactory
	nodeCommunicator  Communicator
	scriptExecutor    execution.ScriptExecutor
	scriptExecMode    ScriptExecutionMode
}

// GetAccount returns the account details at the latest sealed block.
// Alias for GetAccountAtLatestBlock
func (b *backendAccounts) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return b.GetAccountAtLatestBlock(ctx, address)
}

// GetAccountAtLatestBlock returns the account details at the latest sealed block.
func (b *backendAccounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	// get the latest sealed header
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest sealed header: %v", err)
	}

	// get the block id of the latest sealed header
	sealedBlockID := sealed.ID()

	account, err := b.getAccountAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return nil, err
	}

	return account, nil
}

// GetAccountAtBlockHeight returns the account details at the given block height
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

	// get the block id of the latest sealed header
	blockID := header.ID()

	account, err := b.getAccountAtBlock(ctx, address, blockID, header.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", blockID)
		return nil, err
	}

	return account, nil
}

// getAccountAtBlock returns the account details at the given block
//
// The data may be sourced from the local storage or from an execution node depending on the nodes's
// configuration and the availability of the data.
func (b *backendAccounts) getAccountAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (*flow.Account, error) {
	if b.scriptExecMode == ScriptExecutionModeExecutionNodesOnly {
		return b.getAccountFromAnyExeNode(ctx, address, blockID)
	}

	account, err := b.getAccountFromLocalStorage(ctx, address, height)

	if err != nil && b.scriptExecMode == ScriptExecutionModeFailover {
		return b.getAccountFromAnyExeNode(ctx, address, blockID)
	}

	return account, err
}

// getAccountFromLocalStorage retrieves the given account from the local storage.
func (b *backendAccounts) getAccountFromLocalStorage(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (*flow.Account, error) {
	// make sure data is available for the requested block
	account, err := b.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height)
	if err != nil {
		if errors.Is(err, ErrDataNotAvailable) {
			return nil, status.Errorf(codes.OutOfRange, "data for block height %d is not available", height)
		}
		if errors.Is(err, storage.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "data not found: %v", err)
		}
		return nil, rpc.ConvertError(err, "failed to get account from local storage", codes.Internal)
	}

	return account, nil
}

// getAccountFromAnyExeNode retrieves the given account from any EN in `execNodes`.
// We attempt querying each EN in sequence. If any EN returns a valid response, then errors from
// other ENs are logged and swallowed. If all ENs fail to return a valid response, then an
// error aggregating all failures is returned.
func (b *backendAccounts) getAccountFromAnyExeNode(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
) (*flow.Account, error) {
	req := &execproto.GetAccountAtBlockIDRequest{
		Address: address.Bytes(),
		BlockId: blockID[:],
	}

	execNodes, err := executionNodesForBlockID(ctx, blockID, b.executionReceipts, b.state, b.log)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to find execution node to query", codes.Internal)
	}

	var resp *execproto.GetAccountAtBlockIDResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.Identity) error {
			var err error
			start := time.Now()

			resp, err = b.tryGetAccount(ctx, node, req)
			duration := time.Since(start)

			lg := b.log.With().
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

// tryGetAccount attempts to get the account from the given execution node.
func (b *backendAccounts) tryGetAccount(
	ctx context.Context,
	execNode *flow.Identity,
	req *execproto.GetAccountAtBlockIDRequest,
) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return execRPCClient.GetAccountAtBlockID(ctx, req)
}
