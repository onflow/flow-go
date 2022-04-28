package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/state_synchronization"
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
	eds     state_synchronization.ExecutionDataService
	headers storage.Headers
	results storage.ExecutionResults

	fetchTimeout           time.Duration
	highestAvailableHeight func() uint64

	// TODO: refactor this to accept a context in AtIndex instead of storing it on the struct.
	// This requires also refactoring jobqueue.Consumer
	ctx irrecoverable.SignalerContext
}

// NewExecutionDataReader creates and returns a ExecutionDataReader.
func NewExecutionDataReader(
	eds state_synchronization.ExecutionDataService,
	headers storage.Headers,
	results storage.ExecutionResults,
	fetchTimeout time.Duration,
	highestAvailableHeight func() uint64,
) *ExecutionDataReader {
	return &ExecutionDataReader{
		eds:                    eds,
		headers:                headers,
		results:                results,
		fetchTimeout:           fetchTimeout,
		highestAvailableHeight: highestAvailableHeight,
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

	// height has not been downloaded, so height is not available yet
	if height > r.highestAvailableHeight() {
		fmt.Printf("execution data reader: height %d > r.highestAvailableHeight() %d\n", height, r.highestAvailableHeight())
		return nil, storage.ErrNotFound
	}

	executionData, err := r.getExecutionData(r.ctx, height)
	if err != nil {
		fmt.Printf("err getting execution data: %v\n", err)
		return nil, err
	}

	return BlockEntryToJob(&BlockEntry{
		BlockID:       executionData.BlockID,
		Height:        height,
		ExecutionData: executionData,
	}), nil
}

// Head returns the highest consecutive block height with downloaded execution data
func (r *ExecutionDataReader) Head() (uint64, error) {
	return r.highestAvailableHeight(), nil
}

// getExecutionData returns the ExecutionData for the given block height.
// This is used by the execution data reader to get the ExecutionData for a block.
func (r *ExecutionDataReader) getExecutionData(signalCtx irrecoverable.SignalerContext, height uint64) (*state_synchronization.ExecutionData, error) {
	header, err := r.headers.ByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup header for height %d: %w", height, err)
	}

	result, err := r.results.ByBlockID(header.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to lookup execution result for block %s: %w", header.ID(), err)
	}

	ctx, cancel := context.WithTimeout(signalCtx, r.fetchTimeout)
	defer cancel()

	executionData, err := r.eds.Get(ctx, result.ExecutionDataID)

	if err != nil {
		return nil, fmt.Errorf("failed to get execution data for block %s: %w", header.ID(), err)
	}

	return executionData, nil
}
