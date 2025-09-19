package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine/common/version"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/storage"
)

// ErrIncompatibleNodeVersion indicates that node version is incompatible with the block version
var ErrIncompatibleNodeVersion = errors.New("node version is incompatible with data for block")

// baseScriptExecutor holds common logic for all ScriptExecutors.
type baseScriptExecutor struct {
	log zerolog.Logger

	// scripts is used to interact with execution state
	scripts *execution.Scripts

	// versionControl provides information about the current version beacon for each block
	versionControl *version.VersionControl

	// minCompatibleHeight and maxCompatibleHeight define block range compatible with node version
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
}

func NewBaseScriptExecutor(log zerolog.Logger, minHeight, maxHeight uint64) *baseScriptExecutor {
	log.Info().
		Uint64("min_height", minHeight).
		Uint64("max_height", maxHeight).
		Msg("base script executor created")

	return &baseScriptExecutor{
		log:                 log,
		minCompatibleHeight: atomic.NewUint64(minHeight),
		maxCompatibleHeight: atomic.NewUint64(maxHeight),
	}
}

// SetMinCompatibleHeight sets the lowest block height (inclusive).
func (b *baseScriptExecutor) SetMinCompatibleHeight(height uint64) {
	b.minCompatibleHeight.Store(height)
	b.log.Info().Uint64("height", height).Msg("minimum compatible height set")
}

// SetMaxCompatibleHeight sets the highest block height (inclusive).
func (b *baseScriptExecutor) SetMaxCompatibleHeight(height uint64) {
	b.maxCompatibleHeight.Store(height)
	b.log.Info().Uint64("height", height).Msg("maximum compatible height set")
}

func (b *baseScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]byte, error) {
	if err := b.checkHeight(height); err != nil {
		return nil, err
	}

	return b.scripts.ExecuteAtBlockHeight(ctx, script, arguments, height, registerSnapshot)
}

func (b *baseScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.Account, error) {
	if err := b.checkHeight(height); err != nil {
		return nil, err
	}

	return b.scripts.GetAccountAtBlockHeight(ctx, address, height, registerSnapshot)
}

func (b *baseScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	if err := b.checkHeight(height); err != nil {
		return 0, err
	}

	return b.scripts.GetAccountBalance(ctx, address, height, registerSnapshot)
}

func (b *baseScriptExecutor) GetAccountAvailableBalance(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) (uint64, error) {
	if err := b.checkHeight(height); err != nil {
		return 0, err
	}

	return b.scripts.GetAccountAvailableBalance(ctx, address, height, registerSnapshot)
}

func (b *baseScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64, registerSnapshot storage.RegisterSnapshotReader) ([]flow.AccountPublicKey, error) {
	if err := b.checkHeight(height); err != nil {
		return nil, err
	}

	return b.scripts.GetAccountKeys(ctx, address, height, registerSnapshot)
}

func (b *baseScriptExecutor) GetAccountKey(ctx context.Context, address flow.Address, keyIndex uint32, height uint64, registerSnapshot storage.RegisterSnapshotReader) (*flow.AccountPublicKey, error) {
	if err := b.checkHeight(height); err != nil {
		return nil, err
	}

	return b.scripts.GetAccountKey(ctx, address, keyIndex, height, registerSnapshot)
}

// checkHeight checks if the provided block height is compatible with the node's version.
//
// It performs several checks:
// 1. Ensures the ScriptExecutor is initialized.
// 2. Ensures the height is within the compatible version range if version control is enabled.
//
// Parameters:
// - height: the block height to check.
//
// Expected errors:
// - ErrIncompatibleNodeVersion if the block height is not compatible with the node version.
func (b *baseScriptExecutor) checkHeight(height uint64) error {
	if height > b.maxCompatibleHeight.Load() || height < b.minCompatibleHeight.Load() {
		return ErrIncompatibleNodeVersion
	}

	// Version control feature could be disabled. In such a case, ignore related functionality.
	if b.versionControl != nil {
		compatible, err := b.versionControl.CompatibleAtBlock(height)
		if err != nil {
			return fmt.Errorf("failed to check compatibility with block height %d: %w", height, err)
		}

		if !compatible {
			return ErrIncompatibleNodeVersion
		}
	}

	return nil
}
