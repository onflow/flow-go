package execution_data

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionDataAdder handles adding execution data to a blobstore
type ExecutionDataAdder interface {
	// AddExecutionData constructs a blob tree for the given ExecutionData and adds it to the
	// blobstore, and then returns the root CID.
	AddExecutionData(ctx context.Context, executionData *ExecutionData) (flow.Identifier, error)
}
