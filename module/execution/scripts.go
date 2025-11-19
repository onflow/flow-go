package execution

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Scripts provides methods for executing Cadence scripts and querying data
// at specific block heights. It ensures that queries are only run for blocks compatible
// with the node's current version.
type Scripts struct {
	log      zerolog.Logger
	executor *query.QueryExecutor
	headers  storage.Headers

	compatibleHeights *CompatibleHeights
}

var _ ScriptExecutor = (*Scripts)(nil)

// NewScripts creates a new Scripts instance.
func NewScripts(
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	chainID flow.ChainID,
	protocolSnapshotProvider protocol.SnapshotExecutionSubsetProvider,
	header storage.Headers,
	queryConf query.QueryConfig,
	derivedChainData *derived.DerivedChainData,
	enableProgramCacheWrites bool,
	compatibleHeights *CompatibleHeights,
) *Scripts {
	vm := fvm.NewVirtualMachine()

	options := computation.DefaultFVMOptions(
		chainID,
		false,
		true,
	)
	blocks := environment.NewBlockFinder(header)
	options = append(options, fvm.WithBlocks(blocks)) // add blocks for getBlocks calls in scripts
	options = append(options, fvm.WithMetricsReporter(metrics))
	options = append(options, fvm.WithAllowProgramCacheWritesInScriptsEnabled(enableProgramCacheWrites))
	vmCtx := fvm.NewContext(options...)

	queryExecutor := query.NewQueryExecutor(
		queryConf,
		log,
		metrics,
		vm,
		vmCtx,
		derivedChainData,
		protocolSnapshotProvider,
	)

	return &Scripts{
		log:               zerolog.New(log).With().Str("component", "script_executor").Logger(),
		executor:          queryExecutor,
		headers:           header,
		compatibleHeights: compatibleHeights,
	}
}

// ExecuteAtBlockHeight executes provided script against the block height.
// A result value is returned encoded as byte array. An error will be returned if script
// doesn't successfully execute.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
//   - [fvmerrors.ErrCodeScriptExecutionCancelledError] - if script execution canceled.
//   - [fvmerrors.ErrCodeScriptExecutionTimedOutError] - if script execution timed out.
//   - [fvmerrors.ErrCodeComputationLimitExceededError] - if script execution computation limit exceeded.
//   - [fvmerrors.ErrCodeMemoryLimitExceededError] - if script execution memory limit exceeded.
//   - [fvmerrors.FailureCodeLedgerFailure] - if the script execution fails due to ledger errors.
func (s *Scripts) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
	registerSnapshot storage.RegisterSnapshotReader,
) ([]byte, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return nil, err
	}

	value, compUsage, err := s.executor.ExecuteScript(ctx, script, arguments, header, snap)
	// TODO: return compUsage when upstream can handle it
	_ = compUsage
	return value, err
}

// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return 0, err
	}

	return s.executor.GetAccountBalance(ctx, address, header, snap)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return 0, err
	}

	return s.executor.GetAccountAvailableBalance(ctx, address, header, snap)
}

// GetAccountKeys returns a public keys of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccountKeys(ctx, address, header, snap)
}

// GetAccountKey returns a public key of Flow account by the provided address, block height and index.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error) {
	header, snap, err := s.getHeaderAndSnapshot(height, registerSnapshot)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccountKey(ctx, address, keyIndex, header, snap)
}

// getHeaderAndSnapshot retrieves the header and storage snapshot for a given block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if no block is finalized at the provided height.
//   - [storage.ErrHeightNotIndexed] - if the requested height is outside the range of indexed blocks.
func (s *Scripts) getHeaderAndSnapshot(
	height uint64,
	registerSnapshot storage.RegisterSnapshotReader,
) (*flow.Header, snapshot.StorageSnapshot, error) {
	err := s.compatibleHeights.Check(height)
	if err != nil {
		return nil, nil, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return header, snap, nil
}
