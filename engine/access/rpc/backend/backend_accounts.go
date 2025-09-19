package backend

import (
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
	"github.com/onflow/flow-go/engine/common/rpc"
	commonrpc "github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/irrecoverable"
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
	scriptExecMode    IndexQueryMode
}

// GetAccount returns the account details at the latest sealed block.
// Alias for GetAccountAtLatestBlock
func (b *backendAccounts) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	return b.GetAccountAtLatestBlock(ctx, address)
}

// GetAccountAtLatestBlock returns the account details at the latest sealed block.
func (b *backendAccounts) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()

	account, err := b.getAccountAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account at blockID: %v", sealedBlockID)
		return nil, err
	}

	return account, nil
}

// GetAccountAtBlockHeight returns the account details at the given block height.
func (b *backendAccounts) GetAccountAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (*flow.Account, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	account, err := b.getAccountAtBlock(ctx, address, blockID, height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account at height: %d", height)
		return nil, err
	}

	return account, nil
}

// GetAccountBalanceAtLatestBlock returns the account balance at the latest sealed block.
func (b *backendAccounts) GetAccountBalanceAtLatestBlock(ctx context.Context, address flow.Address) (uint64, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return 0, err
	}

	sealedBlockID := sealed.ID()

	balance, err := b.getAccountBalanceAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", sealedBlockID)
		return 0, err
	}

	return balance, nil
}

// GetAccountBalanceAtBlockHeight returns the account balance at the given block height.
func (b *backendAccounts) GetAccountBalanceAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) (uint64, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return 0, rpc.ConvertStorageError(err)
	}

	balance, err := b.getAccountBalanceAtBlock(ctx, address, blockID, height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account balance at height: %v", height)
		return 0, err
	}

	return balance, nil
}

// GetAccountKeyAtLatestBlock returns the account public key at the latest sealed block.
func (b *backendAccounts) GetAccountKeyAtLatestBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
) (*flow.AccountPublicKey, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()

	accountKey, err := b.getAccountKeyAtBlock(ctx, address, keyIndex, sealedBlockID, sealed.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", sealedBlockID)
		return nil, err
	}

	return accountKey, nil
}

// GetAccountKeysAtLatestBlock returns the account public keys at the latest sealed block.
func (b *backendAccounts) GetAccountKeysAtLatestBlock(
	ctx context.Context,
	address flow.Address,
) ([]flow.AccountPublicKey, error) {
	sealed, err := b.state.Sealed().Head()
	if err != nil {
		err := irrecoverable.NewExceptionf("failed to lookup sealed header: %w", err)
		irrecoverable.Throw(ctx, err)
		return nil, err
	}

	sealedBlockID := sealed.ID()
	accountKeys, err := b.getAccountKeysAtBlock(ctx, address, sealedBlockID, sealed.Height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", sealedBlockID)
		return nil, err
	}

	return accountKeys, nil

}

// GetAccountKeyAtBlockHeight returns the account public key by key index at the given block height.
func (b *backendAccounts) GetAccountKeyAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	height uint64,
) (*flow.AccountPublicKey, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	accountKey, err := b.getAccountKeyAtBlock(ctx, address, keyIndex, blockID, height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account key at height: %v", height)
		return nil, err
	}

	return accountKey, nil
}

// GetAccountKeysAtBlockHeight returns the account public keys at the given block height.
func (b *backendAccounts) GetAccountKeysAtBlockHeight(
	ctx context.Context,
	address flow.Address,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	blockID, err := b.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, rpc.ConvertStorageError(err)
	}

	accountKeys, err := b.getAccountKeysAtBlock(ctx, address, blockID, height)
	if err != nil {
		b.log.Debug().Err(err).Msgf("failed to get account keys at height: %v", height)
		return nil, err
	}

	return accountKeys, nil

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
	switch b.scriptExecMode {
	case IndexQueryModeExecutionNodesOnly:
		return b.getAccountFromAnyExeNode(ctx, address, blockID)

	case IndexQueryModeLocalOnly:
		return b.getAccountFromLocalStorage(ctx, address, height)

	case IndexQueryModeFailover:
		localResult, localErr := b.getAccountFromLocalStorage(ctx, address, height)
		if localErr == nil {
			return localResult, nil
		}
		execResult, execErr := b.getAccountFromAnyExeNode(ctx, address, blockID)

		b.compareAccountResults(execResult, execErr, localResult, localErr, blockID, address)

		return execResult, execErr

	case IndexQueryModeCompare:
		execResult, execErr := b.getAccountFromAnyExeNode(ctx, address, blockID)
		// Only compare actual get account errors from the EN, not system errors
		if execErr != nil && !isInvalidArgumentError(execErr) {
			return nil, execErr
		}
		localResult, localErr := b.getAccountFromLocalStorage(ctx, address, height)

		b.compareAccountResults(execResult, execErr, localResult, localErr, blockID, address)

		// always return EN results
		return execResult, execErr

	default:
		return nil, status.Errorf(codes.Internal, "unknown execution mode: %v", b.scriptExecMode)
	}
}

func (b *backendAccounts) getAccountBalanceAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) (uint64, error) {
	switch b.scriptExecMode {
	case IndexQueryModeExecutionNodesOnly:
		account, err := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
			return 0, err
		}
		return account.Balance, nil

	case IndexQueryModeLocalOnly:
		accountBalance, err := b.scriptExecutor.GetAccountBalance(ctx, address, height)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account balance at blockID: %v", blockID)
			return 0, err
		}

		return accountBalance, nil
	case IndexQueryModeFailover:
		localAccountBalance, localErr := b.scriptExecutor.GetAccountBalance(ctx, address, height)
		if localErr == nil {
			return localAccountBalance, nil
		}
		execResult, execErr := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if execErr != nil {
			return 0, execErr
		}

		return execResult.Balance, nil

	default:
		return 0, status.Errorf(codes.Internal, "unknown execution mode: %v", b.scriptExecMode)
	}
}

func (b *backendAccounts) getAccountKeysAtBlock(
	ctx context.Context,
	address flow.Address,
	blockID flow.Identifier,
	height uint64,
) ([]flow.AccountPublicKey, error) {
	switch b.scriptExecMode {
	case IndexQueryModeExecutionNodesOnly:
		account, err := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account keys at blockID: %v", blockID)
			return nil, err
		}
		return account.Keys, nil
	case IndexQueryModeLocalOnly:
		accountKeys, err := b.scriptExecutor.GetAccountKeys(ctx, address, height)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account keys at height: %d", height)
			return nil, err
		}

		return accountKeys, nil
	case IndexQueryModeFailover:
		localAccountKeys, localErr := b.scriptExecutor.GetAccountKeys(ctx, address, height)
		if localErr == nil {
			return localAccountKeys, nil
		}

		account, err := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if err != nil {
			return nil, err
		}

		return account.Keys, nil

	default:
		return nil, status.Errorf(codes.Internal, "unknown execution mode: %v", b.scriptExecMode)
	}

}

func (b *backendAccounts) getAccountKeyAtBlock(
	ctx context.Context,
	address flow.Address,
	keyIndex uint32,
	blockID flow.Identifier,
	height uint64,
) (*flow.AccountPublicKey, error) {
	switch b.scriptExecMode {
	case IndexQueryModeExecutionNodesOnly:
		account, err := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", blockID)
			return nil, err
		}

		for _, key := range account.Keys {
			if key.Index == keyIndex {
				return &key, nil
			}
		}

		return nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)
	case IndexQueryModeLocalOnly:
		accountKey, err := b.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account key at height: %d", height)
			return nil, err
		}

		return accountKey, nil
	case IndexQueryModeFailover:
		localAccountKey, localErr := b.scriptExecutor.GetAccountKey(ctx, address, keyIndex, height)
		if localErr == nil {
			return localAccountKey, nil
		}

		account, err := b.getAccountFromAnyExeNode(ctx, address, blockID)
		if err != nil {
			b.log.Debug().Err(err).Msgf("failed to get account key at blockID: %v", blockID)
			return nil, err
		}

		for _, key := range account.Keys {
			if key.Index == keyIndex {
				return &key, nil
			}
		}

		return nil, status.Errorf(codes.NotFound, "failed to get account key by index: %d", keyIndex)

	default:
		return nil, status.Errorf(codes.Internal, "unknown execution mode: %v", b.scriptExecMode)
	}
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
		return nil, convertAccountError(err, address, height)
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

	execNodes, err := commonrpc.ExecutionNodesForBlockID(
		ctx,
		blockID,
		b.executionReceipts,
		b.state,
		b.log,
		preferredENIdentifiers,
		fixedENIdentifiers,
	)
	if err != nil {
		return nil, rpc.ConvertError(err, "failed to find execution node to query", codes.Internal)
	}

	var resp *execproto.GetAccountAtBlockIDResponse
	errToReturn := b.nodeCommunicator.CallAvailableNode(
		execNodes,
		func(node *flow.IdentitySkeleton) error {
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
	execNode *flow.IdentitySkeleton,
	req *execproto.GetAccountAtBlockIDRequest,
) (*execproto.GetAccountAtBlockIDResponse, error) {
	execRPCClient, closer, err := b.connFactory.GetExecutionAPIClient(execNode.Address)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return execRPCClient.GetAccountAtBlockID(ctx, req)
}

// compareAccountResults compares the result and error returned from local and remote getAccount calls
// and logs the results if they are different
func (b *backendAccounts) compareAccountResults(
	execNodeResult *flow.Account,
	execErr error,
	localResult *flow.Account,
	localErr error,
	blockID flow.Identifier,
	address flow.Address,
) {
	if b.log.GetLevel() > zerolog.DebugLevel {
		return
	}

	lgCtx := b.log.With().
		Hex("block_id", blockID[:]).
		Str("address", address.String())

	// errors are different
	if execErr != localErr {
		lgCtx = lgCtx.
			AnErr("execution_node_error", execErr).
			AnErr("local_error", localErr)

		lg := lgCtx.Logger()
		lg.Debug().Msg("errors from getting account on local and EN do not match")
		return
	}

	// both errors are nil, compare the accounts
	if execErr == nil {
		lgCtx, ok := compareAccountsLogger(execNodeResult, localResult, lgCtx)
		if !ok {
			lg := lgCtx.Logger()
			lg.Debug().Msg("accounts from local and EN do not match")
		}
	}
}

// compareAccountsLogger compares accounts produced by the execution node and local storage and
// return a logger configured to log the differences
func compareAccountsLogger(exec, local *flow.Account, lgCtx zerolog.Context) (zerolog.Context, bool) {
	different := false

	if exec.Address != local.Address {
		lgCtx = lgCtx.
			Str("exec_node_address", exec.Address.String()).
			Str("local_address", local.Address.String())
		different = true
	}

	if exec.Balance != local.Balance {
		lgCtx = lgCtx.
			Uint64("exec_node_balance", exec.Balance).
			Uint64("local_balance", local.Balance)
		different = true
	}

	contractListMatches := true
	if len(exec.Contracts) != len(local.Contracts) {
		lgCtx = lgCtx.
			Int("exec_node_contract_count", len(exec.Contracts)).
			Int("local_contract_count", len(local.Contracts))
		contractListMatches = false
		different = true
	}

	missingContracts := zerolog.Arr()
	mismatchContracts := zerolog.Arr()

	for name, execContract := range exec.Contracts {
		localContract, ok := local.Contracts[name]

		if !ok {
			missingContracts.Str(name)
			contractListMatches = false
			different = true
		}

		if !bytes.Equal(execContract, localContract) {
			mismatchContracts.Str(name)
			different = true
		}
	}

	lgCtx = lgCtx.
		Array("missing_contracts", missingContracts).
		Array("mismatch_contracts", mismatchContracts)

	// only check if there were any missing
	if !contractListMatches {
		extraContracts := zerolog.Arr()
		for name := range local.Contracts {
			if _, ok := exec.Contracts[name]; !ok {
				extraContracts.Str(name)
				different = true
			}
		}
		lgCtx = lgCtx.Array("extra_contracts", extraContracts)
	}

	if len(exec.Keys) != len(local.Keys) {
		lgCtx = lgCtx.
			Int("exec_node_key_count", len(exec.Keys)).
			Int("local_key_count", len(local.Keys))
		different = true
	}

	mismatchKeys := zerolog.Arr()

	for i, execKey := range exec.Keys {
		localKey := local.Keys[i]

		if !execKey.PublicKey.Equals(localKey.PublicKey) {
			mismatchKeys.Uint32(execKey.Index)
			different = true
		}
	}

	lgCtx = lgCtx.Array("mismatch_keys", mismatchKeys)

	return lgCtx, !different
}

// convertAccountError converts the script execution error to a gRPC error
func convertAccountError(err error, address flow.Address, height uint64) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "account with address %s not found: %v", address, err)
	}

	if fvmerrors.IsAccountNotFoundError(err) {
		return status.Errorf(codes.NotFound, "account not found")
	}

	return rpc.ConvertIndexError(err, height, "failed to get account")
}
