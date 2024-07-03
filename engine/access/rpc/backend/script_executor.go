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
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
)

// ErrIncompatibleNodeVersion indicates that node version is incompatible with the block version
var ErrIncompatibleNodeVersion = errors.New("node version is incompatible with block version")

type ScriptExecutor struct {
	log zerolog.Logger

	// scriptExecutor is used to interact with execution state
	scriptExecutor *execution.Scripts

	// indexReporter provides information about the current state of the execution state indexer.
	indexReporter state_synchronization.IndexReporter

	// versionControl provides information about the current version beacon for each block
	versionControl *version.VersionControl

	// initialized is used to signal that the index and executor are ready
	initialized *atomic.Bool

	// minCompatibleHeight and maxCompatibleHeight are used to limit the block range that can be queried using local execution
	// to ensure only blocks that are compatible with the node's current software version are allowed.
	// Note: this is a temporary solution for cadence/fvm upgrades while version beacon support is added
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
}

func NewScriptExecutor(log zerolog.Logger, versionControl *version.VersionControl, minHeight, maxHeight uint64) *ScriptExecutor {
	logger := log.With().Str("component", "script-executor").Logger()
	logger.Info().
		Uint64("min_height", minHeight).
		Uint64("max_height", maxHeight).
		Msg("script executor created")

	return &ScriptExecutor{
		log:                 logger,
		versionControl:      versionControl,
		initialized:         atomic.NewBool(false),
		minCompatibleHeight: atomic.NewUint64(minHeight),
		maxCompatibleHeight: atomic.NewUint64(maxHeight),
	}
}

// SetMinCompatibleHeight sets the lowest block height (inclusive) that can be queried using local execution
// Use this to limit the executable block range supported by the node's current software version.
func (s *ScriptExecutor) SetMinCompatibleHeight(height uint64) {
	s.minCompatibleHeight.Store(height)
	s.log.Info().Uint64("height", height).Msg("minimum compatible height set")
}

// SetMaxCompatibleHeight sets the highest block height (inclusive) that can be queried using local execution
// Use this to limit the executable block range supported by the node's current software version.
func (s *ScriptExecutor) SetMaxCompatibleHeight(height uint64) {
	s.maxCompatibleHeight.Store(height)
	s.log.Info().Uint64("height", height).Msg("maximum compatible height set")
}

// Initialize initializes the indexReporter and script executor
// This method can be called at any time after the ScriptExecutor object is created. Any requests
// made to the other methods will return storage.ErrHeightNotIndexed until this method is called.
func (s *ScriptExecutor) Initialize(indexReporter state_synchronization.IndexReporter, scriptExecutor *execution.Scripts) error {
	if s.initialized.CompareAndSwap(false, true) {
		s.log.Info().Msg("script executor initialized")
		s.indexReporter = indexReporter
		s.scriptExecutor = scriptExecutor
		return nil
	}
	return fmt.Errorf("script executor already initialized")
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - storage.ErrHeightNotIndexed if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64) ([]byte, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.scriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - storage.ErrHeightNotIndexed if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height)
}

// GetAccountBalance returns
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *ScriptExecutor) GetAccountBalance(ctx context.Context, address flow.Address, height uint64) (uint64, error) {
	if err := s.checkHeight(height); err != nil {
		return 0, err
	}

	return s.scriptExecutor.GetAccountBalance(ctx, address, height)
}

// GetAccountKeys returns
// Expected errors:
// - Script execution related errors
// - storage.ErrHeightNotIndexed if the data for the block height is not available
func (s *ScriptExecutor) GetAccountKeys(ctx context.Context, address flow.Address, height uint64) ([]flow.AccountPublicKey, error) {
	if err := s.checkHeight(height); err != nil {
		return nil, err
	}

	return s.scriptExecutor.GetAccountKeys(ctx, address, height)
}

func (s *ScriptExecutor) checkHeight(height uint64) error {
	if !s.initialized.Load() {
		return fmt.Errorf("%w: script executor not initialized", storage.ErrHeightNotIndexed)
	}

	// Version control feature could be disabled. In such a case, ignore related functionality.
	if s.versionControl != nil {
		compatible, err := s.versionControl.CompatibleAtBlock(height)
		if err != nil {
			return err
		}

		if !compatible {
			return ErrIncompatibleNodeVersion
		}
	}

	highestHeight, err := s.indexReporter.HighestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get highest indexed height: %w", err)
	}
	if height > highestHeight {
		return fmt.Errorf("%w: block not indexed yet", storage.ErrHeightNotIndexed)
	}

	lowestHeight, err := s.indexReporter.LowestIndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get lowest indexed height: %w", err)
	}
	if height < lowestHeight {
		return fmt.Errorf("%w: block is before lowest indexed height", storage.ErrHeightNotIndexed)
	}

	if height > s.maxCompatibleHeight.Load() || height < s.minCompatibleHeight.Load() {
		return fmt.Errorf("%w: node software is not compatible with version required to executed block", storage.ErrHeightNotIndexed)
	}

	return nil
}
