package backend

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization"
)

type ScriptExecutor struct {
	log zerolog.Logger

	// scriptExecutor is used to interact with execution state
	scriptExecutor *execution.Scripts

	// indexReporter provides information about the current state of the execution state indexer.
	indexReporter state_synchronization.IndexReporter

	// initialized is used to signal that the index and executor are ready
	initialized *atomic.Bool

	// minCompatibleHeight and maxCompatibleHeight are used to limit the block range that can be queried using local execution
	// to ensure only blocks that are compatible with the node's current software version are allowed.
	// Note: this is a temporary solution for cadence/fvm upgrades while version beacon support is added
	minCompatibleHeight *atomic.Uint64
	maxCompatibleHeight *atomic.Uint64
}

func NewScriptExecutor(log zerolog.Logger, minHeight, maxHeight uint64) *ScriptExecutor {
	logger := log.With().Str("component", "script-executor").Logger()
	logger.Info().
		Uint64("min_height", minHeight).
		Uint64("max_height", maxHeight).
		Msg("script executor created")

	return &ScriptExecutor{
		log:                 logger,
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

// InitReporter initializes the indexReporter and script executor
// This method can be called at any time after the ScriptExecutor object is created. Any requests
// made to the other methods will return execution.ErrDataNotAvailable until this method is called.
func (s *ScriptExecutor) InitReporter(indexReporter state_synchronization.IndexReporter, scriptExecutor *execution.Scripts) {
	if s.initialized.CompareAndSwap(false, true) {
		s.log.Info().Msg("script executor initialized")
		s.indexReporter = indexReporter
		s.scriptExecutor = scriptExecutor
	}
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - execution.ErrDataNotAvailable if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64) ([]byte, error) {
	if err := s.checkDataAvailable(height); err != nil {
		return nil, err
	}

	data, _, err := s.scriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height)
	return data, err
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - execution.ErrDataNotAvailable if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	if err := s.checkDataAvailable(height); err != nil {
		return nil, err
	}

	return s.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height)
}

func (s *ScriptExecutor) checkDataAvailable(height uint64) error {
	if !s.initialized.Load() {
		return fmt.Errorf("%w: script executor not initialized", execution.ErrDataNotAvailable)
	}

	if height > s.indexReporter.HighestIndexedHeight() {
		return fmt.Errorf("%w: block not indexed yet", execution.ErrDataNotAvailable)
	}

	if height < s.indexReporter.LowestIndexedHeight() {
		return fmt.Errorf("%w: block is before lowest indexed height", execution.ErrDataNotAvailable)
	}

	if height > s.maxCompatibleHeight.Load() || height < s.minCompatibleHeight.Load() {
		return fmt.Errorf("%w: node software is not compatible with version required to executed block", execution.ErrDataNotAvailable)
	}

	return nil
}
