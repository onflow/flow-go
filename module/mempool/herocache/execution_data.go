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
	t := &BlockExecutionData{
		c: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(limit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "block_execution_data").Logger(),
					collector))),
	}

	return t
}

// Has checks whether the block execution data with the given hash is currently in
// the memory pool.
func (t BlockExecutionData) Has(id flow.Identifier) bool {
	return t.c.Has(id)
}

// Add adds a block execution data to the mempool.
func (t *BlockExecutionData) Add(ed *execution_data.BlockExecutionDataEntity) bool {
	entity := internal.NewWrappedEntity(ed.BlockID, ed)
	return t.c.Add(*entity)
}

// ByID returns the block execution data with the given ID from the mempool.
func (t BlockExecutionData) ByID(txID flow.Identifier) (*execution_data.BlockExecutionDataEntity, bool) {
	entity, exists := t.c.ByID(txID)
	if !exists {
		return nil, false
	}

	return unwrap(entity), true
}

// All returns all block execution data from the mempool. Since it is using the HeroCache, All guarantees returning
// all block execution data in the same order as they are added.
func (t BlockExecutionData) All() []*execution_data.BlockExecutionDataEntity {
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
func (t BlockExecutionData) Size() uint {
	return t.c.Size()
}

// Remove removes block execution data from mempool.
func (t *BlockExecutionData) Remove(id flow.Identifier) bool {
	return t.c.Remove(id)
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
