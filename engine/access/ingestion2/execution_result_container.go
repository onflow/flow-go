package ingestion2

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// ExecutionResultContainer represents a set of ExecutionReceipts all committing to the same
// ExecutionResult. As an ExecutionResult contains the Block ID, all results with the same
// ID must be for the same block. For optimized storage, we only store the result once.
// Mathematically, an ExecutionResultContainer struct represents an Equivalence Class of
// Execution Receipts.
// Not safe for concurrent access.
type ExecutionResultContainer struct {
	receipts    map[flow.Identifier]*flow.ExecutionReceiptMeta // map from ExecutionReceipt.ID -> ExecutionReceiptMeta
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
	pipeline    optimistic_sync.Pipeline
	enqueued    *atomic.Bool
}

// NewExecutionResultContainer creates a new instance of ExecutionResultContainer.
//
// Parameters:
//   - result: the execution result to store
//   - header: the block header associated with the result
//   - pipeline: the pipeline to process the result
//
// Returns:
//   - *ExecutionResultContainer: the newly created container
//   - error: any error that occurred during creation
//
// No errors are expected during normal operation.
//
// Concurrency safety:
//   - Not safe for concurrent access
func NewExecutionResultContainer(result *flow.ExecutionResult, header *flow.Header, pipeline optimistic_sync.Pipeline) (*ExecutionResultContainer, error) {
	// sanity check: initial result must be for block
	if header.ID() != result.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	return &ExecutionResultContainer{
		receipts:    make(map[flow.Identifier]*flow.ExecutionReceiptMeta),
		result:      result,
		resultID:    result.ID(),
		blockHeader: header,
		pipeline:    pipeline,
		enqueued:    atomic.NewBool(false),
	}, nil
}

// IsEnqueued returns true if the container is enqueued for processing.
//
// Returns:
//   - bool: true if the container is enqueued, false otherwise
//
// Concurrency safety:
//   - Safe for concurrent access
func (c *ExecutionResultContainer) IsEnqueued() bool {
	return c.enqueued.Load()
}

// SetEnqueued sets the container as enqueued for processing.
//
// Returns:
//   - bool: true if the container was successfully set as enqueued, false if it was already enqueued
//
// Concurrency safety:
//   - Safe for concurrent access
func (c *ExecutionResultContainer) SetEnqueued() bool {
	return c.enqueued.CompareAndSwap(false, true)
}

// AddReceipt adds the given execution receipt to the container.
//
// Parameters:
//   - receipt: the execution receipt to add
//
// Returns:
//   - uint: number of receipts added (0 or 1)
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) AddReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	if receipt.ExecutionResult.ID() != c.resultID {
		return 0, fmt.Errorf("cannot add receipt for different result")
	}

	receiptID := receipt.ID()
	if c.Has(receiptID) {
		return 0, nil
	}
	c.receipts[receiptID] = receipt.Meta()
	return 1, nil
}

// AddReceipts adds multiple execution receipts to the container.
//
// Parameters:
//   - receipts: variadic list of execution receipts to add
//
// Returns:
//   - uint: total number of receipts added
//   - error: any error that occurred during the operation
//
// No errors are expected during normal operation.
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) AddReceipts(receipts ...*flow.ExecutionReceipt) (uint, error) {
	receiptsAdded := uint(0)
	for i := 0; i < len(receipts); i++ {
		added, err := c.AddReceipt(receipts[i])
		if err != nil {
			return receiptsAdded, fmt.Errorf("failed to add receipt (%x) to equivalence class: %w", receipts[i].ID(), err)
		}
		receiptsAdded += added
	}
	return receiptsAdded, nil
}

// Has checks if a receipt with the given ID exists in the container.
//
// Parameters:
//   - receiptID: the ID of the receipt to check
//
// Returns:
//   - bool: true if the receipt exists, false otherwise
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) Has(receiptID flow.Identifier) bool {
	_, found := c.receipts[receiptID]
	return found
}

// Size returns the number of receipts in the container.
//
// Returns:
//   - uint: the number of receipts stored
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) Size() uint {
	return uint(len(c.receipts))
}

// Pipeline returns the pipeline associated with this container.
//
// Returns:
//   - *Pipeline: the associated pipeline, or nil if no pipeline is set
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) Pipeline() optimistic_sync.Pipeline {
	return c.pipeline
}

/* Methods implementing LevelledForest's Vertex interface */

// VertexID returns the ID of this vertex in the forest.
//
// Returns:
//   - flow.Identifier: the result ID
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) VertexID() flow.Identifier { return c.resultID }

// Level returns the view of the block associated with this result.
//
// Returns:
//   - uint64: the block view
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) Level() uint64 { return c.blockHeader.View }

// Parent returns the parent result ID and its block view.
//
// Returns:
//   - flow.Identifier: the parent result ID
//   - uint64: the parent block view
//
// Concurrency safety:
//   - Not safe for concurrent access
func (c *ExecutionResultContainer) Parent() (flow.Identifier, uint64) {
	return c.result.PreviousResultID, c.blockHeader.ParentView
}
