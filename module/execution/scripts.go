package execution

import (
	"context"
	"errors"

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

type Scripts struct {
	executor *query.QueryExecutor
	headers  storage.Headers
}

func NewScripts(
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	chainID flow.ChainID,
	protocolSnapshotProvider protocol.SnapshotExecutionSubsetProvider,
	header storage.Headers,
	queryConf query.QueryConfig,
	derivedChainData *derived.DerivedChainData,
	enableProgramCacheWrites bool,
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
		executor: queryExecutor,
		headers:  header,
	}
}

// ExecuteAtBlockHeight executes provided script against the block height.
// A result value is returned encoded as byte array. An error will be returned if script
// doesn't successfully execute.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
	register storage.RegisterIndexReader,
) ([]byte, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return nil, err
	}

	value, compUsage, err := s.executor.ExecuteScript(ctx, script, arguments, header, snap)
	// TODO: return compUsage when upstream can handle it
	_ = compUsage
	return value, err
}

// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, register storage.RegisterIndexReader) (*flow.Account, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountBalance(ctx context.Context, address flow.Address, height uint64, register storage.RegisterIndexReader) (uint64, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return 0, err
	}

	return s.executor.GetAccountBalance(ctx, address, header, snap)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, register storage.RegisterIndexReader) (uint64, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return 0, err
	}

	return s.executor.GetAccountAvailableBalance(ctx, address, header, snap)
}

// GetAccountKeys returns a public keys of Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountKeys(ctx context.Context, address flow.Address, height uint64, register storage.RegisterIndexReader) ([]flow.AccountPublicKey, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccountKeys(ctx, address, header, snap)
}

// GetAccountKey returns a public key of Flow account by the provided address, block height and index.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, register storage.RegisterIndexReader) (*flow.AccountPublicKey, error) {
	snap, header, err := s.snapshotWithBlock(register, height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccountKey(ctx, address, keyIndex, header, snap)
}

// snapshotWithBlock is a common function for executing scripts and get account functionality.
// It creates a storage snapshot that is needed by the FVM to execute scripts.
func (s *Scripts) snapshotWithBlock(register storage.RegisterIndexReader, height uint64) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, nil, err
	}

	storageSnapshot := snapshot.NewReadFuncStorageSnapshot(func(ID flow.RegisterID) (flow.RegisterValue, error) {
		value, err := register.Get(ID, height)
		if err != nil {
			// only return an error if the error doesn't match the not found error, since we have
			// to gracefully handle not found values and instead assign nil, that is because the script executor
			// expects that behaviour
			if errors.Is(err, storage.ErrNotFound) {
				return nil, nil
			}

			return nil, err
		}

		return value, nil
	})

	return storageSnapshot, header, nil
}
