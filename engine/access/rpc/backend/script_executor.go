package backend

import (
	"context"
	"math"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/state_synchronization"
)

type ScriptExecutor struct {
	// scriptExecutor is used to interact with execution state
	scriptExecutor *execution.Scripts

	// indexReporter provides information about the current state of the execution state indexer.
	indexReporter state_synchronization.IndexReporter

	// initialized is used to signal that the index and executor are ready
	initialized *atomic.Bool

	// minHeight and maxHeight are used to limit the block range that can be queried using local execution
	// to ensure only blocks that are compatible with the node's current software version are allowed.
	// Note: this is a temporary solution for cadence/fvm upgrades while version beacon support is added
	minHeight *atomic.Uint64
	maxHeight *atomic.Uint64
}

func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		initialized: atomic.NewBool(false),
		minHeight:   atomic.NewUint64(0),
		maxHeight:   atomic.NewUint64(math.MaxUint64),
	}
}

// SetMinExecutableHeight sets the lowest block height (inclusive) that can be queried using local execution
// Use this to limit the executable block range supported by the node's current software version.
func (s *ScriptExecutor) SetMinExecutableHeight(height uint64) {
	s.minHeight.Store(height)
}

// SetMaxExecutableHeight sets the highest block height (inclusive) that can be queried using local execution
// Use this to limit the executable block range supported by the node's current software version.
func (s *ScriptExecutor) SetMaxExecutableHeight(height uint64) {
	s.maxHeight.Store(height)
}

// InitReporter initializes the indexReporter and script executor
// This method can be called at any time after the ScriptExecutor object is created. Any requests
// made to the other methods will return execution.ErrDataNotAvailable until this method is called.
func (s *ScriptExecutor) InitReporter(indexReporter state_synchronization.IndexReporter, scriptExecutor *execution.Scripts) {
	if s.initialized.CompareAndSwap(false, true) {
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
	if !s.isDataAvailable(height) {
		return nil, execution.ErrDataNotAvailable
	}

	return s.scriptExecutor.ExecuteAtBlockHeight(ctx, script, arguments, height)
}

// GetAccountAtBlockHeight returns the account at the provided block height from a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the account or block height is not found
//   - execution.ErrDataNotAvailable if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	if !s.isDataAvailable(height) {
		return nil, execution.ErrDataNotAvailable
	}

	return s.scriptExecutor.GetAccountAtBlockHeight(ctx, address, height)
}

func (s *ScriptExecutor) isDataAvailable(height uint64) bool {
	return s.initialized.Load() &&
		height <= s.indexReporter.HighestIndexedHeight() &&
		height >= s.indexReporter.LowestIndexedHeight() &&
		height <= s.maxHeight.Load() &&
		height >= s.minHeight.Load()
}
