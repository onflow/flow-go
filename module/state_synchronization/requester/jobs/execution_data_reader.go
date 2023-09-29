package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data/cache"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/storage"
)

// BlockEntry represents a block that's tracked by the ExecutionDataRequester
type BlockEntry struct {
	BlockID       flow.Identifier
	Height        uint64
	ExecutionData *execution_data.BlockExecutionDataEntity
}

var _ module.Jobs = (*ExecutionDataReader)(nil)

// ExecutionDataReader provides an abstraction for consumers to read blocks as job.
type ExecutionDataReader struct {
	store *cache.ExecutionDataCache

	fetchTimeout             time.Duration
	highestConsecutiveHeight func() (uint64, error)

	// TODO: refactor this to accept a context in AtIndex instead of storing it on the struct.
	// This requires also refactoring jobqueue.Consumer
	ctx irrecoverable.SignalerContext
}

// NewExecutionDataReader creates and returns a ExecutionDataReader.
func NewExecutionDataReader(
	store *cache.ExecutionDataCache,
	fetchTimeout time.Duration,
	highestConsecutiveHeight func() (uint64, error),
) *ExecutionDataReader {
	return &ExecutionDataReader{
		store:                    store,
		fetchTimeout:             fetchTimeout,
		highestConsecutiveHeight: highestConsecutiveHeight,
	}
}

// AddContext adds a context to the execution data reader
// TODO: this is an anti-pattern, refactor this to accept a context in AtIndex instead of storing
// it on the struct.
func (r *ExecutionDataReader) AddContext(ctx irrecoverable.SignalerContext) {
	r.ctx = ctx
}

// AtIndex returns the block entry job at the given height, or storage.ErrNotFound.
// Any other error is unexpected
func (r *ExecutionDataReader) AtIndex(height uint64) (module.Job, error) {
	if r.ctx == nil {
		return nil, fmt.Errorf("execution data reader is not initialized")
	}

	// data for the requested height or a lower height, has not been downloaded yet.
	highestHeight, err := r.highestConsecutiveHeight()
	if err != nil {
		return nil, fmt.Errorf("failed to get highest height: %w", err)
	}

	if height > highestHeight {
		return nil, storage.ErrNotFound
	}

	ctx, cancel := context.WithTimeout(r.ctx, r.fetchTimeout)
	defer cancel()

	executionData, err := r.store.ByHeight(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("failed to get execution data for height %d: %w", height, err)
	}

	return BlockEntryToJob(&BlockEntry{
		BlockID:       executionData.BlockID,
		Height:        height,
		ExecutionData: executionData,
	}), nil
}

// Head returns the highest consecutive block height with downloaded execution data
func (r *ExecutionDataReader) Head() (uint64, error) {
	return r.highestConsecutiveHeight()
}
