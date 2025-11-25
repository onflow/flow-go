package ingestion2

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/forest"
)

// ErrIncompatibleReceipt is returned when an execution receipt is added to a container for a
// different execution result.
var ErrIncompatibleReceipt = errors.New("incompatible execution receipt")

// ExecutionResultContainer implements the Vertex interface from the LevelledForest module.
// It represents a set of ExecutionReceipts all committing to the same ExecutionResult. As
// an ExecutionResult contains the Block ID, all results with the same ID must be for the
// same block. For optimized storage, we only store the result once. Mathematically, an
// ExecutionResultContainer struct represents an Equivalence Class of Execution Receipts.
type ExecutionResultContainer struct {
	// conceptually immutable values; can be read without concurrency protection:
	result      *flow.ExecutionResult // execution result that all receipts in this container commit to
	resultID    flow.Identifier       // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header          // header of the block which the result is for

	// inherently concurrency-safe values; can be read without concurrency protection:
	pipeline     optimistic_sync.Pipeline
	resultStatus ResultStatusTracker

	// values requiring concurrency protection
	mu       sync.RWMutex
	receipts map[flow.Identifier]*flow.ExecutionReceiptStub // map from ExecutionReceipt.ID -> ExecutionReceiptStub
}

var _ forest.Vertex = (*ExecutionResultContainer)(nil)

// NewExecutionResultContainer creates a new instance of ExecutionResultContainer. Conceptually,
// this is a set of execution receipts all committing to the *same* execution `result` for the
// specified block. The set of receipts can be empty, which is useful to represent sealed results
// for example, where we don't care anymore with Execution Node published the result.
//
// No error returns are expected during normal operation.
func NewExecutionResultContainer(
	result *flow.ExecutionResult,
	header *flow.Header,
	pipeline optimistic_sync.Pipeline,
) (*ExecutionResultContainer, error) {
	// sanity check: initial result must be for block
	if header.ID() != result.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	return &ExecutionResultContainer{
		receipts:     make(map[flow.Identifier]*flow.ExecutionReceiptStub),
		result:       result,
		resultID:     result.ID(),
		blockHeader:  header,
		pipeline:     pipeline,
		resultStatus: NewResultStatusTracker(ResultForCertifiedBlock),
	}, nil
}

// AddReceipt adds the given execution receipt to the container.
// Returns the number of receipts added (0 if receipt already exists, 1 if added).
//
// Expected error returns during normal operations:
//   - [ErrIncompatibleReceipt]: if the receipt's execution result is different from the container's result ID
func (c *ExecutionResultContainer) AddReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.addReceipt(receipt)
}

// AddReceipts adds execution receipts to the container.
// Returns the total number of receipts added.
//
// Expected error returns during normal operations:
//   - [ErrIncompatibleReceipt]: if any of the receipts is for a result that is different from the container's result ID
func (c *ExecutionResultContainer) AddReceipts(receipts ...*flow.ExecutionReceipt) (uint, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	receiptsAdded := uint(0)
	for _, receipt := range receipts {
		added, err := c.addReceipt(receipt)
		if err != nil {
			return receiptsAdded, fmt.Errorf("failed to add receipt (%x) to equivalence class: %w", receipt.ID(), err)
		}
		receiptsAdded += added
	}
	return receiptsAdded, nil
}

// addReceipt adds a single receipt to the container.
// Returns the number of receipts added (0 if receipt already exists, 1 if added).
// CAUTION: not concurrency safe! Caller must hold a lock.
//
// Expected error returns during normal operations:
//   - [ErrIncompatibleReceipt]: if the receipt's execution result is different from the container's result ID
func (c *ExecutionResultContainer) addReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	resultID := receipt.ExecutionResult.ID()
	if resultID != c.resultID {
		return 0, fmt.Errorf("receipt is for result %v, while ExecutionResultContainer pertains to result %v: %w",
			resultID, c.resultID, ErrIncompatibleReceipt)
	}

	receiptID := receipt.ID()
	if c.has(receiptID) {
		return 0, nil
	}
	c.receipts[receiptID] = receipt.Stub()
	return 1, nil
}

// Has returns whether a receipt with the given ID exists in the container.
func (c *ExecutionResultContainer) Has(receiptID flow.Identifier) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.has(receiptID)
}

// has returns whether a receipt with the given ID exists in the container.
// CAUTION: not concurrency safe! Caller must hold a lock.
func (c *ExecutionResultContainer) has(receiptID flow.Identifier) bool {
	_, found := c.receipts[receiptID]
	return found
}

// Size returns the number of receipts in the container.
func (c *ExecutionResultContainer) Size() uint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return uint(len(c.receipts))
}

// Result returns the execution result for this container.
func (c *ExecutionResultContainer) Result() *flow.ExecutionResult {
	// No locking is required here since the result is immutable after instantiation.
	return c.result
}

// ResultID returns the ID of the execution result for this container.
func (c *ExecutionResultContainer) ResultID() flow.Identifier {
	// No locking is required here since the resultID is immutable after instantiation.
	return c.resultID
}

// BlockHeader returns the header of the block executed by this result.
func (c *ExecutionResultContainer) BlockHeader() *flow.Header {
	// No locking is required here since the blockHeader is immutable after instantiation.
	return c.blockHeader
}

// BlockView returns the view of the block executed by this result.
func (c *ExecutionResultContainer) BlockView() uint64 {
	// No locking is required here since the blockHeader is immutable after instantiation.
	return c.blockHeader.View
}

// Pipeline returns the pipeline associated with this container.
func (c *ExecutionResultContainer) Pipeline() optimistic_sync.Pipeline {
	// No locking is required here since the pipeline is immutable after instantiation.
	return c.pipeline
}

// ResultStatus returns the status of this result.
func (c *ExecutionResultContainer) ResultStatus() ResultStatus {
	return c.resultStatus.Value()
}

// SetResultStatus sets the status of this result.
//
// No error returns expected during normal operations.
func (c *ExecutionResultContainer) SetResultStatus(resultStatus ResultStatus) error {
	success, oldValue := c.resultStatus.Set(resultStatus)
	if !success {
		return fmt.Errorf("invalid result status transition: %s -> %s", oldValue, resultStatus)
	}

	if resultStatus == ResultSealed {
		c.pipeline.SetSealed()
	}
	return nil
}

// Abandon marks the result as orphaned and abandons the pipeline.
//
// No error returns expected during normal operations.
func (c *ExecutionResultContainer) Abandon() error {
	if err := c.SetResultStatus(ResultOrphaned); err != nil {
		return fmt.Errorf("failed to abandon result: %w", err)
	}
	c.pipeline.Abandon()
	return nil
}

// Methods implementing LevelledForest's Vertex interface

// VertexID returns the execution ID's result. The ExecutionResultContainer is a vertex in
// the LevelledForest, where we use the result ID to identify the vertex.
func (c *ExecutionResultContainer) VertexID() flow.Identifier {
	// No locking is required here since the resultID is immutable after instantiation.
	return c.resultID
}

// Level returns the view of the block executed by this result. The ExecutionResultContainer is a
// vertex in the LevelledForest, where we use the result ID to identify the vertex.
func (c *ExecutionResultContainer) Level() uint64 {
	// No locking is required here since the blockHeader is immutable after instantiation.
	return c.blockHeader.View
}

// Parent returns the parent result's ID and its block view. This pair `(flow.Identifier, uint64)`
// uniquely identifies the parent vertex in the LevelledForest.
func (c *ExecutionResultContainer) Parent() (flow.Identifier, uint64) {
	// No locking is required here since the result and blockHeader are immutable after instantiation.
	return c.result.PreviousResultID, c.blockHeader.ParentView
}
