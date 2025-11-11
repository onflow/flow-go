package subscription

import (
	"context"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// HeightTracker is an interface for a tracker that provides base GetStartHeight method related
// to both blocks and execution data tracking.
type HeightTracker interface {
	// GetStartHeightFromBlockID returns the start height based on the provided starting block ID.
	// If the start block is the root block, skip it and begin from the next block.
	//
	// Parameters:
	// - startBlockID: The identifier of the starting block.
	//
	// Returns:
	// - uint64: The start height associated with the provided block ID.
	// - error: An error indicating any issues with retrieving the start height.
	//
	// Expected errors during normal operation:
	// - codes.NotFound - if the block was not found in storage
	// - codes.Internal - for any other error
	GetStartHeightFromBlockID(flow.Identifier) (uint64, error)

	// GetStartHeightFromHeight returns the start height based on the provided starting block height.
	// If the start block is the root block, skip it and begin from the next block.
	//
	// Parameters:
	// - startHeight: The height of the starting block.
	//
	// Returns:
	// - uint64: The start height associated with the provided block height.
	// - error: An error indicating any issues with retrieving the start height.
	//
	// Expected errors during normal operation:
	// - codes.InvalidArgument   - if the start height is less than the root block height.
	// - codes.NotFound  - if the header was not found in storage.
	GetStartHeightFromHeight(uint64) (uint64, error)

	// GetStartHeightFromLatest returns the start height based on the latest sealed block.
	// If the start block is the root block, skip it and begin from the next block.
	//
	// Parameters:
	// - ctx: Context for the operation.
	//
	// No errors are expected during normal operation.
	GetStartHeightFromLatest(context.Context) (uint64, error)

	// GetStartHeight returns the start height to use when searching.
	// Only one of startBlockID and startHeight may be set. Otherwise, an InvalidArgument error is returned.
	// If a block is provided and does not exist, a NotFound error is returned.
	// If neither startBlockID nor startHeight is provided, the latest sealed block is used.
	// If the start block is the root block, skip it and begin from the next block.
	//
	// Parameters:
	// - ctx: Context for the operation.
	// - startBlockID: The identifier of the starting block. If provided, startHeight should be 0.
	// - startHeight: The height of the starting block. If provided, startBlockID should be flow.ZeroID.
	//
	// Returns:
	// - uint64: The start height for searching.
	// - error: An error indicating the result of the operation, if any.
	//
	// Expected errors during normal operation:
	// - codes.InvalidArgument - if both startBlockID and startHeight are provided,
	// if the start height is less than the root block height,
	// if the start height is out of bounds based on indexed heights (when index is used).
	// - codes.NotFound - if a block is provided and does not exist.
	// - codes.Internal - if there is an internal error.
	GetStartHeight(context.Context, flow.Identifier, uint64) (uint64, error)
}

// BlockTracker is an interface for tracking blocks and handling block-related operations.
type BlockTracker interface {
	HeightTracker

	// GetHighestHeight returns the highest height based on the specified block status which
	// could be only BlockStatusSealed or BlockStatusFinalized.
	// No errors are expected during normal operation.
	GetHighestHeight(flow.BlockStatus) (uint64, error)

	// ProcessOnFinalizedBlock drives the subscription logic when a block is finalized.
	// The input to this callback is treated as trusted. This method should be executed on
	// `OnFinalizedBlock` notifications from the node-internal consensus instance.
	// No errors are expected during normal operation.
	ProcessOnFinalizedBlock() error
}

// ExecutionDataTracker is an interface for tracking the highest consecutive block height for which we have received a
// new Execution Data notification
type ExecutionDataTracker interface {
	HeightTracker

	// GetHighestHeight returns the highest height that we have consecutive execution data for.
	GetHighestHeight() uint64

	// OnExecutionData is used to notify the tracker when a new execution data is received.
	OnExecutionData(*execution_data.BlockExecutionDataEntity)
}
