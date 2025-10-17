package execution

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// ErrIncompatibleNodeVersion indicates that node version is incompatible with the block version.
var ErrIncompatibleNodeVersion = errors.New("node version is incompatible with data for block")

// Scripts provides methods for executing Cadence scripts and querying data
// at specific block heights. It ensures that queries are only run for blocks compatible
// with the node's current version.
type Scripts struct {
	log      zerolog.Logger
	executor *query.QueryExecutor
	headers  storage.Headers

	// versionControl provides information about the current version beacon for each block
	versionControl *version.VersionControl

	// minCompatibleHeight and maxCompatibleHeight are used to limit the block range that can be queried using local execution
	// to ensure only blocks that are compatible with the node's current software version are allowed.
	// Note: this is a temporary solution for cadence/fvm upgrades while version beacon support is added
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
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
	minHeight uint64,
	maxHeight uint64,
	versionControl *version.VersionControl,
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
		log:                 zerolog.New(log).With().Str("component", "script_executor").Logger(),
		executor:            queryExecutor,
		headers:             header,
		minCompatibleHeight: atomic.NewUint64(minHeight),
		maxCompatibleHeight: atomic.NewUint64(maxHeight),
		versionControl:      versionControl,
	}
}

// ExecuteAtBlockHeight executes provided script against the block height.
// A result value is returned encoded as byte array. An error will be returned if script
// doesn't successfully execute.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) ExecuteAtBlockHeight(
	ctx context.Context,
	script []byte,
	arguments [][]byte,
	height uint64,
	registerSnapshot storage.RegisterSnapshotReader,
) ([]byte, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return nil, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return nil, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
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
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return nil, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return nil, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return s.executor.GetAccount(ctx, address, header, snap)
}

// GetAccountBalance returns a balance of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return 0, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return 0, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return 0, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return s.executor.GetAccountBalance(ctx, address, header, snap)
}

// GetAccountAvailableBalance returns an available balance of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return 0, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return 0, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return 0, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return s.executor.GetAccountAvailableBalance(ctx, address, header, snap)
}

// GetAccountKeys returns a public keys of Flow account by the provided address and block height.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return nil, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return nil, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return s.executor.GetAccountKeys(ctx, address, header, snap)
}

// GetAccountKey returns a public key of Flow account by the provided address, block height and index.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
//   - [storage.ErrNotFound] - if block or registerSnapshot value at height was not found.
//   - [storage.ErrHeightNotIndexed] - if the requested height is below the first indexed height or above the latest indexed height.
func (s *Scripts) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error) {
	err := s.verifyHeight(height)
	if err != nil {
		return nil, fmt.Errorf("block height is not compatible with the node's version: %w", err)
	}
	header, err := s.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("could not get header for height %d: %w", height, err)
	}
	snap, err := registerSnapshot.StorageSnapshot(height)
	if err != nil {
		return nil, fmt.Errorf("could not get storage snapshot for height %d: %w", height, err)
	}

	return s.executor.GetAccountKey(ctx, address, keyIndex, header, snap)
}

// SetMinCompatibleHeight sets the lowest block height (inclusive).
func (s *Scripts) SetMinCompatibleHeight(height uint64) {
	s.minCompatibleHeight.Store(height)
	s.log.Info().Uint64("height", height).Msg("minimum compatible height set")
}

// SetMaxCompatibleHeight sets the highest block height (inclusive).
func (s *Scripts) SetMaxCompatibleHeight(height uint64) {
	s.maxCompatibleHeight.Store(height)
	s.log.Info().Uint64("height", height).Msg("maximum compatible height set")
}

// verifyHeight checks whether the given block height is compatible with the node's version.
// It performs checks:
// 1. Ensures the height is within the configured minCompatibleHeight and maxCompatibleHeight.
// 2. If version control is enabled, ensures the height is compatible with the node's version beacon.
//
// Expected error returns during normal operation:
//   - [version.ErrOutOfRange] - if incoming block height is higher that last handled block height.
//   - [execution.ErrIncompatibleNodeVersion] - if the block height is not compatible with the node version.
func (s *Scripts) verifyHeight(height uint64) error {
	if height > s.maxCompatibleHeight.Load() || height < s.minCompatibleHeight.Load() {
		return ErrIncompatibleNodeVersion
	}

	// Version control feature could be disabled. In such a case, ignore related functionality.
	if s.versionControl != nil {
		compatible, err := s.versionControl.CompatibleAtBlock(height)
		if err != nil {
			return fmt.Errorf("failed to check compatibility with block height %d: %w", height, err)
		}

		if !compatible {
			return ErrIncompatibleNodeVersion
		}
	}

	return nil
}
