package state_synchronization

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/onflow/flow-go/model/flow"
)

const DefaultMaxBlobSize = 1 << 20 // 1MiB

// ExecutionDataAdder handles adding execution data to a blobservice
type ExecutionDataAdder interface {
	// AddChunkExecutionData constructs a blob tree for the given ChunkExecutionData
	// and adds it to the blobservice, and then returns the root CID.
	AddChunkExecutionData(
		ctx context.Context,
		blockID flow.Identifier,
		chunkIndex int,
		ced *ChunkExecutionData,
	) (cid.Cid, error)

	// AddExecutionDataRoot creates an ExecutionDataRoot and adds it to the blobservice,
	// and then returns the ID.
	AddExecutionDataRoot(
		ctx context.Context,
		blockID flow.Identifier,
		chunkExecutionDataIDs []cid.Cid,
	) (flow.Identifier, error)
}

// ExecutionDataGetter handles getting execution data from a blobservice
type ExecutionDataGetter interface {
	// GetChunkExecutionDatas gets the ExecutionData for the given root CID from the blobservice.
	// The returned error will be:
	// - MalformedDataError if some level of the blob tree cannot be properly deserialized
	// - BlobSizeLimitExceededError if any blob in the blob tree exceeds the maximum blob size
	// - BlobNotFoundError if some CID in the blob tree could not be found from the blobservice
	// - MismatchedBlockIDError if the block ID of the Execution Data root does not match the given block ID
	GetChunkExecutionDatas(
		ctx context.Context,
		blockID flow.Identifier,
		rootID flow.Identifier,
	) ([]*ChunkExecutionData, error)
}
