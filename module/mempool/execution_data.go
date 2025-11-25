package mempool

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// ExecutionData represents a concurrency-safe memory pool for BlockExecutionData.
type ExecutionData Mempool[flow.Identifier, *execution_data.BlockExecutionDataEntity]
