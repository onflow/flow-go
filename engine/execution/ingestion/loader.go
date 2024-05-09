package ingestion

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

type BlockLoader interface {
	LoadUnexecuted(ctx context.Context) ([]flow.Identifier, error)
}
