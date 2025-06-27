package ingestion2

import (
	"errors"
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
)

// ErrIncompatibleReceipt is returned when an execution receipt is added to a container for a
// different execution result.
var ErrIncompatibleReceipt = errors.New("incompatible execution receipt")

// ExecutionResultContainer implements the Vertex interface from the LevelledForest module.
// It represents a set of ExecutionReceipts all committing to the same ExecutionResult. As
// an ExecutionResult contains the Block ID, all results with the same ID must be for the
// same block. For optimized storage, we only store the result once. Mathematically, an
// ExecutionResultContainer struct represents an Equivalence Class of Execution Receipts.
//
// CAUTION: not concurrency safe!
type ExecutionResultContainer struct {
	receipts    map[flow.Identifier]*flow.ExecutionReceiptMeta // map from ExecutionReceipt.ID -> ExecutionReceiptMeta
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
	pipeline    optimistic_sync.Pipeline
	enqueued    *atomic.Bool
}

// NewExecutionResultContainer creates a new instance of ExecutionResultContainer. Conceptually, this is a set of
// execution receipts all committing to the *same* execution `result` for the specified block.
// CAUTION: ExecutionResultContainer is not concurrency safe!
//
// No errors are expected during normal operation.
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
// Returns the number of receipts added (0 if receipt already exists, 1 if added).
// CAUTION: not concurrency safe!
//
// Expected errors during normal operations:
//   - ErrIncompatibleReceipt: if the receipt's execution result is different from the container's result ID
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
func (c *ExecutionResultContainer) AddReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	if receipt.ExecutionResult.ID() != c.resultID {
		return 0, fmt.Errorf("receipt is for result %v, while ExecutionResultContainer pertains to result %v: %w",
			receipt.ExecutionResult.ID(), c.resultID, ErrIncompatibleReceipt)
	}

	receiptID := receipt.ID()
	if c.Has(receiptID) {
		return 0, nil
	}
	c.receipts[receiptID] = receipt.Meta()
	return 1, nil
}

// AddReceipts adds execution receipts to the container.
// Returns the total number of receipts added.
// CAUTION: not concurrency safe!
//
// Expected errors during normal operations:
//   - ErrIncompatibleReceipt: if any of the receipts is for a result that is different from the container's result ID
//   - All other errors are potential indicators of bugs or corrupted internal state (continuation impossible)
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

// Has returns whether a receipt with the given ID exists in the container.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) Has(receiptID flow.Identifier) bool {
	_, found := c.receipts[receiptID]
	return found
}

// Size returns the number of receipts in the container.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) Size() uint {
	return uint(len(c.receipts))
}

// Pipeline returns the pipeline associated with this container.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) Pipeline() optimistic_sync.Pipeline {
	return c.pipeline
}

// Methods implementing LevelledForest's Vertex interface

// VertexID returns the execution ID's result. The ExecutionResultContainer is a vertex in
// the LevelledForest, where we use the result ID to identify the vertex.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) VertexID() flow.Identifier { return c.resultID }

// Level returns the view of the block executed by this result. The ExecutionResultContainer is a
// vertex in the LevelledForest, where we use the result ID to identify the vertex.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) Level() uint64 { return c.blockHeader.View }

// Parent returns the parent result's ID and its block view. This pair `(flow.Identifier, uint64)`
// uniquely identifies the parent vertex in the LevelledForest.
// CAUTION: not concurrency safe!
func (c *ExecutionResultContainer) Parent() (flow.Identifier, uint64) {
	return c.result.PreviousResultID, c.blockHeader.ParentView
}
