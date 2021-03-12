package consensus

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ReceiptsOfSameResult represents a set of ExecutionReceipt all committing to the same
// ExecutionResult. As an ExecutionResult contains the Block ID, all results with the same
// ID must be for the same block. For optimized storage, we only store the result once.
// Mathematically, a ReceiptsOfSameResult struct represents an Equivalence Class of
// Execution Receipts.
// Implements LevelledForest's Vertex interface.
type ReceiptsOfSameResult struct {
	receipts    map[flow.Identifier]*flow.ExecutionReceiptMeta // map from ExecutionReceipt.ID -> ExecutionReceiptMeta
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
}

// NewReceiptsOfSameResult instantiates an empty Equivalence Class (without any receipts)
func NewReceiptsOfSameResult(result *flow.ExecutionResult, block *flow.Header) (*ReceiptsOfSameResult, error) {
	//sanity check: initial result should be for block
	if block.ID() != result.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	// construct ReceiptsOfSameResult only containing initialReceipt
	rcpts := make(map[flow.Identifier]*flow.ExecutionReceiptMeta)
	rs := &ReceiptsOfSameResult{
		receipts:    rcpts,
		result:      result,
		resultID:    result.ID(),
		blockHeader: block,
	}
	return rs, nil
}

// AddReceipt adds the receipt to the ReceiptsOfSameResult (if not already stored).
// Returns:
//  * uint: number of receipts added (consistent API with AddReceipts()),
//          Possible values: 0 or 1
//  * error in case of unforeseen problems
func (rsr *ReceiptsOfSameResult) AddReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	if receipt.ExecutionResult.ID() != rsr.resultID {
		return 0, errors.New("cannot add receipt for different result")
	}

	receiptID := receipt.ID()
	if rsr.Has(receiptID) {
		return 0, nil
	}
	rsr.receipts[receipt.ID()] = receipt.Meta()
	return 1, nil
}

// AddReceipts adds the receipts to the ReceiptsOfSameResult (the ones not already stored).
// Returns:
//  * uint: number of receipts added
//  * error in case of unforeseen problems
func (rsr *ReceiptsOfSameResult) AddReceipts(receipts ...*flow.ExecutionReceipt) (uint, error) {
	receiptsAdded := uint(0)
	for i := 0; i < len(receipts); i++ {
		added, err := rsr.AddReceipt(receipts[i])
		if err != nil {
			return receiptsAdded, fmt.Errorf("failed to add receipt (%x) to equivalence class: %w", receipts[i].ID(), err)
		}
		receiptsAdded += added
	}
	return receiptsAdded, nil
}

func (rsr *ReceiptsOfSameResult) Has(receiptID flow.Identifier) bool {
	_, found := rsr.receipts[receiptID]
	return found
}

// Size returns the number of receipts in the equivalence class (i.e. the number of
// receipts known for that particular result)
func (rsr *ReceiptsOfSameResult) Size() uint {
	return uint(len(rsr.receipts))
}

/* Methods implementing LevelledForest's Vertex interface */

func (rsr *ReceiptsOfSameResult) VertexID() flow.Identifier { return rsr.resultID }
func (rsr *ReceiptsOfSameResult) Level() uint64             { return rsr.blockHeader.Height }
func (rsr *ReceiptsOfSameResult) Parent() (flow.Identifier, uint64) {
	return rsr.result.PreviousResultID, rsr.blockHeader.Height - 1
}
