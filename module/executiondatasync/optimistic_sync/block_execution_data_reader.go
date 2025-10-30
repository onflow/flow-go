package optimistic_sync

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// BlockExecutionDataReader provides access to per-block execution data sourced from a particular snapshot.
type BlockExecutionDataReader interface {
	// ByBlockID returns the execution data for the given block ID.
	//
	// Expected errors during normal operation:
	// - [storage.ErrNotFound]: if a seal or execution result is not available for the block
	// - [BlobNotFoundError]: if some CID in the blob tree could not be found from the blobstore
	// - [MalformedDataError]: if some level of the blob tree cannot be properly deserialized
	// - [BlobSizeLimitExceededError]: if some blob in the blob tree exceeds the maximum allowed size
	ByBlockID(ctx context.Context, blockID flow.Identifier) (*execution_data.BlockExecutionDataEntity, error)
}
