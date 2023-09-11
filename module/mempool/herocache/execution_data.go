package herocache

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/herocache/internal"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type BlockExecutionData struct {
	c *stdmap.Backend
}

// NewBlockExecutionData implements a block execution data mempool based on hero cache.
func NewBlockExecutionData(limit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *BlockExecutionData {
	return &BlockExecutionData{
		c: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(limit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "block_execution_data").Logger(),
					collector))),
	}
}

// Has checks whether the block execution data for the given block ID is currently in
// the memory pool.
func (t *BlockExecutionData) Has(blockID flow.Identifier) bool {
	return t.c.Has(blockID)
}

// Add adds a block execution data to the mempool, keyed by block ID.
// It returns false if the execution data was already in the mempool.
func (t *BlockExecutionData) Add(ed *execution_data.BlockExecutionDataEntity) bool {
	entity := internal.NewWrappedEntity(ed.BlockID, ed)
	return t.c.Add(*entity)
}

// ByID returns the block execution data for the given block ID from the mempool.
// It returns false if the execution data was not found in the mempool.
func (t *BlockExecutionData) ByID(blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
	entity, exists := t.c.ByID(blockID)
	if !exists {
		return nil, false
	}

	return unwrap(entity), true
}

// All returns all block execution data from the mempool. Since it is using the HeroCache, All guarantees returning
// all block execution data in the same order as they are added.
func (t *BlockExecutionData) All() []*execution_data.BlockExecutionDataEntity {
	entities := t.c.All()
	eds := make([]*execution_data.BlockExecutionDataEntity, 0, len(entities))
	for _, entity := range entities {
		eds = append(eds, unwrap(entity))
	}
	return eds
}

// Clear removes all block execution data stored in this mempool.
func (t *BlockExecutionData) Clear() {
	t.c.Clear()
}

// Size returns total number of stored block execution data.
func (t *BlockExecutionData) Size() uint {
	return t.c.Size()
}

// Remove removes block execution data from mempool by block ID.
// It returns true if the execution data was known and removed.
func (t *BlockExecutionData) Remove(blockID flow.Identifier) bool {
	return t.c.Remove(blockID)
}

// unwrap converts an internal.WrappedEntity to a BlockExecutionDataEntity.
func unwrap(entity flow.Entity) *execution_data.BlockExecutionDataEntity {
	wrappedEntity, ok := entity.(internal.WrappedEntity)
	if !ok {
		panic(fmt.Sprintf("invalid wrapped entity in block execution data pool (%T)", entity))
	}

	ed, ok := wrappedEntity.Entity.(*execution_data.BlockExecutionDataEntity)
	if !ok {
		panic(fmt.Sprintf("invalid entity in block execution data pool (%T)", wrappedEntity.Entity))
	}

	return ed
}
