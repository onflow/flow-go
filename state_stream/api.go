package state_stream

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

type API interface {
	GetExecutionDataByBlockID(ctx context.Context, blockID flow.Identifier) (error, error)
}
