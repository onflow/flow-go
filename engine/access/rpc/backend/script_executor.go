package backend

import (
	"context"
	"sync"

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

	// init is used to ensure that the object is initialized only once
	init sync.Once
}

func NewScriptExecutor() *ScriptExecutor {
	return &ScriptExecutor{
		initialized: atomic.NewBool(false),
	}
}

// InitReporter initializes the indexReporter and script executor
// This method can be called at any time after the ScriptExecutor object is created. Any requests
// made to the other methods will return execution.ErrDataNotAvailable until this method is called.
func (s *ScriptExecutor) InitReporter(indexReporter state_synchronization.IndexReporter, scriptExecutor *execution.Scripts) {
	s.init.Do(func() {
		defer s.initialized.Store(true)
		s.indexReporter = indexReporter
		s.scriptExecutor = scriptExecutor
	})
}

// ExecuteAtBlockHeight executes provided script at the provided block height against a local execution state.
//
// Expected errors:
//   - storage.ErrNotFound if the register or block height is not found
//   - execution.ErrDataNotAvailable if the data for the block height is not available. this could be because
//     the height is not within the index block range, or the index is not ready.
func (s *ScriptExecutor) ExecuteAtBlockHeight(ctx context.Context, script []byte, arguments [][]byte, height uint64) ([]byte, uint64, error) {
	if !s.isDataAvailable(height) {
		return nil, 0, execution.ErrDataNotAvailable
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
	return s.initialized.Load() && height <= s.indexReporter.HighestIndexedHeight() && height >= s.indexReporter.LowestIndexedHeight()
}
