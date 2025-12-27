package execution_data

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
)

// ExecutionDataCache provides a read-through cache for execution data.
// All methods are safe for concurrent use.
type ExecutionDataCache interface {
	// ByID returns the execution data for the given ExecutionDataID.
	//
	// Expected error returns during normal operation:
	//   - [BlobNotFoundError]: if some CID in the blob tree could not be found from the blobstore
	//   - [MalformedDataError]: if some level of the blob tree cannot be properly deserialized
	//   - [BlobSizeLimitExceededError]: if some blob in the blob tree exceeds the maximum allowed size
	ByID(ctx context.Context, executionDataID flow.Identifier) (*BlockExecutionDataEntity, error)

	// ByBlockID returns the execution data for the given block ID.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if a seal or execution result is not available for the block
	//   - [BlobNotFoundError]: if some CID in the blob tree could not be found from the blobstore
	//   - [MalformedDataError]: if some level of the blob tree cannot be properly deserialized
	//   - [BlobSizeLimitExceededError]: if some blob in the blob tree exceeds the maximum allowed size
	ByBlockID(ctx context.Context, blockID flow.Identifier) (*BlockExecutionDataEntity, error)

	// ByHeight returns the execution data for the given block height.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if a seal or execution result is not available for the block
	//   - [BlobNotFoundError]: if some CID in the blob tree could not be found from the blobstore
	//   - [MalformedDataError]: if some level of the blob tree cannot be properly deserialized
	//   - [BlobSizeLimitExceededError]: if some blob in the blob tree exceeds the maximum allowed size
	ByHeight(ctx context.Context, height uint64) (*BlockExecutionDataEntity, error)

	// LookupID returns the ExecutionDataID for the given block ID.
	//
	// Expected error returns during normal operation:
	//   - [storage.ErrNotFound]: if a seal or execution result is not available for the block
	LookupID(blockID flow.Identifier) (flow.Identifier, error)
}
