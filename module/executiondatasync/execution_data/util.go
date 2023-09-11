package execution_data

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/blobs"
)

// CalculateID calculates the root ID of the given execution data without storing any data.
// No errors are expected during normal operation.
func CalculateID(ctx context.Context, execData *BlockExecutionData, serializer Serializer) (flow.Identifier, error) {
	executionDatastore := NewExecutionDataStore(blobs.NewNoopBlobstore(), serializer)

	id, err := executionDatastore.Add(ctx, execData)
	if err != nil {
		return flow.ZeroID, err
	}

	return id, nil
}
