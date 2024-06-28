package execution

import (
	"context"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// RegisterAtHeight returns register value for provided register ID at the block height.
// Even if the register wasn't indexed at the provided height, returns the highest height the register was indexed at.
// If the register with the ID was not indexed at all return nil value and no error.
// Expected errors:
// - storage.ErrHeightNotIndexed if the given height was not indexed yet or lower than the first indexed height.
type RegisterAtHeight func(ID flow.RegisterID, height uint64) (flow.RegisterValue, error)

type ScriptExecutor interface {
	// ExecuteAtBlockHeight executes provided script against the block height.
	// A result value is returned encoded as byte array. An error will be returned if script
	// doesn't successfully execute.
	// Expected errors:
	// - storage.ErrNotFound if block or register value at height was not found.
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	ExecuteAtBlockHeight(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		height uint64,
	) ([]byte, error)

	// GetAccountAtBlockHeight returns a Flow account by the provided address and block height.
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error)

	// GetAccountBalance returns
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error)

	// GetAccountKeys returns
	// Expected errors:
	// - storage.ErrHeightNotIndexed if the data for the block height is not available
	GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error)
}

var _ ScriptExecutor = (*Scripts)(nil)

type Scripts struct {
	executor         *query.QueryExecutor
	headers          storage.Headers
	registerAtHeight RegisterAtHeight
}

func NewScripts(
	log zerolog.Logger,
	metrics module.ExecutionMetrics,
	chainID flow.ChainID,
	entropy query.EntropyProviderPerBlock,
	header storage.Headers,
	registerAtHeight RegisterAtHeight,
	queryConf query.QueryConfig,
	derivedChainData *derived.DerivedChainData,
	enableProgramCacheWrites bool,
) *Scripts {
	vm := fvm.NewVirtualMachine()

	options := computation.DefaultFVMOptions(chainID, false, false)
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
		entropy,
	)

	return &Scripts{
		executor:         queryExecutor,
		headers:          header,
		registerAtHeight: registerAtHeight,
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
) ([]byte, error) {

	snap, header, err := s.snapshotWithBlock(height)
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
func (s *Scripts) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return 0, err
	}

	return s.executor.GetAccountBalance(ctx, address, header, snap)
}

// GetAccountKeys returns a public keys of Flow account by the provided address and block height.
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *Scripts) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	snap, header, err := s.snapshotWithBlock(height)
	if err != nil {
		return nil, err
	}

	return s.executor.GetAccountKeys(ctx, address, header, snap)
}

// snapshotWithBlock is a common function for executing scripts and get account functionality.
// It creates a storage snapshot that is needed by the FVM to execute scripts.
func (s *Scripts) snapshotWithBlock(height uint64) (snapshot.StorageSnapshot, *flow.Header, error) {
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, nil, err
	}

	storageSnapshot := snapshot.NewReadFuncStorageSnapshot(func(ID flow.RegisterID) (flow.RegisterValue, error) {
		return s.registerAtHeight(ID, height)
	})

	return storageSnapshot, header, nil
}
