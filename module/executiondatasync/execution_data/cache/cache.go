package cache

import (
	"context"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/storage"
)

type ExecutionDataCache struct {
	execution_data.ExecutionDataGetter

	headers storage.Headers
	seals   storage.Seals
	results storage.ExecutionResults
	cache   mempool.ExecutionData
}

func NewExecutionDataCache(
	backend execution_data.ExecutionDataGetter,
	headers storage.Headers,
	seals storage.Seals,
	results storage.ExecutionResults,
	cache mempool.ExecutionData,
) *ExecutionDataCache {
	return &ExecutionDataCache{
		ExecutionDataGetter: backend,

		headers: headers,
		seals:   seals,
		results: results,
		cache:   cache,
	}
}

// ByID returns the execution data for the given ExecutionDataID.
func (c *ExecutionDataCache) ByID(ctx context.Context, executionDataID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error) {
	execData, err := c.Get(ctx, executionDataID)
	if err != nil {
		return nil, err
	}

	return execution_data.NewBlockExecutionDataEntity(executionDataID, execData), nil
}

func (c *ExecutionDataCache) ByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error) {
	if execData, ok := c.cache.ByID(blockID); ok {
		return execData, nil
	}

	executionDataID, err := c.LookupID(blockID)
	if err != nil {
		return nil, err
	}

	execData, err := c.Get(ctx, executionDataID)
	if err != nil {
		return nil, err
	}

	execDataEntity := execution_data.NewBlockExecutionDataEntity(executionDataID, execData)

	_ = c.cache.Add(execDataEntity)

	return execDataEntity, nil
}

func (c *ExecutionDataCache) ByHeight(ctx context.Context, height uint64) (*execution_data.BlockExecutionDataEntity, error) {
	blockID, err := c.headers.BlockIDByHeight(height)
	if err != nil {
		return nil, err
	}

	return c.ByBlockID(ctx, blockID)
}

// LookupID returns the ExecutionDataID for the given block ID.
// Errors:
// - storage.ErrNotFound if a seal or execution result is not available for the block
func (c *ExecutionDataCache) LookupID(blockID flow.Identifier) (flow.Identifier, error) {
	seal, err := c.seals.FinalizedSealForBlock(blockID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("failed to lookup seal for block %s: %w", blockID, err)
	}

	result, err := c.results.ByID(seal.ResultID)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("failed to lookup execution result for block %s: %w", blockID, err)
	}

	return result.ExecutionDataID, nil
}
