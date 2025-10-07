package optimistic_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

type BlockExecutionDataReader interface {
	ByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error)
}
