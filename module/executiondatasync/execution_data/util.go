package execution_data

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
)

func CalculateID(ctx context.Context, execData *BlockExecutionData, serializer Serializer) (flow.Identifier, error) {
	executionDatastore := NewExecutionDataStore(&blobs.NoopBlobstore{}, serializer)

	id, err := executionDatastore.AddExecutionData(ctx, execData)
	if err != nil {
		return flow.ZeroID, err
	}

	return id, nil
}
