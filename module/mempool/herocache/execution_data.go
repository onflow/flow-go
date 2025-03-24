package herocache

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// BlockExecutionData implements the block execution data memory pool.
// Stored execution data are keyed by block id.
type BlockExecutionData struct {
	*stdmap.Backend[flow.Identifier, *execution_data.BlockExecutionDataEntity]
}

// NewBlockExecutionData implements a block execution data mempool based on hero cache.
func NewBlockExecutionData(limit uint32, logger zerolog.Logger, collector module.HeroCacheMetrics) *BlockExecutionData {
	return &BlockExecutionData{
		stdmap.NewBackend(
			stdmap.WithMutableBackData[flow.Identifier, *execution_data.BlockExecutionDataEntity](
				herocache.NewCache[*execution_data.BlockExecutionDataEntity](limit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "block_execution_data").Logger(),
					collector,
				),
			),
		),
	}
}
