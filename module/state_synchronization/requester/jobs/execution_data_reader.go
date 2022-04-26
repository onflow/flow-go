package jobs

import (
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/storage"
)

// BlockEntry represents a block that's tracked by the ExecutionDataRequester
type BlockEntry struct {
	BlockID       flow.Identifier
	Height        uint64
	ExecutionData *state_synchronization.ExecutionData
}

// ExecutionDataReader provides an abstraction for consumers to read blocks as job.
type ExecutionDataReader struct {
	component.Component
	cm *component.ComponentManager

	head func() uint64

	// TODO: doc, read must be concurrency safe
	read ReadExecutionData

	maxCachedEntries uint64
	cache            map[uint64]*state_synchronization.ExecutionData

	// TODO: refactor this to accept a context in AtIndex instead of storing it on the struct.
	// This requires also refactoring jobqueue.Consumer
	ctx irrecoverable.SignalerContext

	mu sync.RWMutex
}

type ReadExecutionData func(irrecoverable.SignalerContext, uint64) (*state_synchronization.ExecutionData, error)

// NewExecutionDataReader creates and returns a ExecutionDataReader.
func NewExecutionDataReader(maxCachedEntries uint64, fetchHead func() uint64, fetchData ReadExecutionData) *ExecutionDataReader {
	r := &ExecutionDataReader{
		head:  fetchHead,
		read:  fetchData,
		cache: make(map[uint64]*state_synchronization.ExecutionData),

		maxCachedEntries: maxCachedEntries,
	}

	r.cm = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			r.ctx = ctx
			ready()
			<-ctx.Done()
		}).
		Build()
	r.Component = r.cm

	return r
}

// AtIndex returns the block entry job at the given index, or storage.ErrNotFound.
// Any other error is unexpected
func (r *ExecutionDataReader) AtIndex(index uint64) (module.Job, error) {
	if !util.CheckClosed(r.Ready()) || util.CheckClosed(r.cm.ShutdownSignal()) {
		// no jobs are available if reader is not ready or is shutting down
		return nil, storage.ErrNotFound
	}

	if index > r.head() {
		return nil, storage.ErrNotFound
	}

	r.mu.RLock()
	executionData, cached := r.cache[index]
	r.mu.RUnlock()

	if !cached {
		// Data is not in the cache, look it up
		var err error
		executionData, err = r.read(r.ctx, index)
		if err != nil {
			return nil, err
		}
	}

	return BlockEntryToJob(&BlockEntry{
		BlockID:       executionData.BlockID,
		Height:        index,
		ExecutionData: executionData,
	}), nil
}

// Head returns the highest consecutive block height with downloaded execution data
func (r *ExecutionDataReader) Head() (uint64, error) {
	return r.head(), nil
}

// Add adds an execution data to the cache
func (r *ExecutionDataReader) Add(index uint64, executionData *state_synchronization.ExecutionData) {
	head := r.head()

	// Don't cache if execution data for the height is already available
	if index <= head {
		return
	}

	// Enforce a maximum cache size
	if index-head > r.maxCachedEntries {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache[index] = executionData
}

// Remove removes an execution data from the cache
func (r *ExecutionDataReader) Remove(index uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.cache, index)
}
