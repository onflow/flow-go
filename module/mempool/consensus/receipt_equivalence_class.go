package consensus

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// ReceiptEquivalenceClass represents a set of ExecutionReceipt all committing to the same
// ExecutionResult. As an ExecutionResult contains the Block ID, all results with the same
// ID must be for the same block. For optimized storage, we only store the result once.
// Implements LevelledForest's Vertex interface.
type ReceiptEquivalenceClass struct {
	receipts    map[flow.Identifier]*flow.ExecutionReceiptMeta // map from ExecutionReceipt.ID -> ExecutionReceiptMeta
	result      *flow.ExecutionResult
	resultID    flow.Identifier // precomputed ID of result to avoid expensive hashing on each call
	blockHeader *flow.Header    // header of the block which the result is for
}

// NewReceiptEquivalenceClass instantiates an empty Equivalence Class (without any receipts)
func NewReceiptEquivalenceClass(result *flow.ExecutionResult, block *flow.Header) (*ReceiptEquivalenceClass, error) {
	//sanity check: initial result should be for block
	if block.ID() != result.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	// construct ReceiptEquivalenceClass only containing initialReceipt
	rcpts := make(map[flow.Identifier]*flow.ExecutionReceiptMeta)
	rs := &ReceiptEquivalenceClass{
		receipts:    rcpts,
		result:      result,
		resultID:    result.ID(),
		blockHeader: block,
	}
	return rs, nil
}

// AddReceipt adds the receipt to the ReceiptEquivalenceClass (if not already stored).
// Returns:
//  * uint: number of receipts added (consistent API with AddReceipts()),
//          Possible values: 0 or 1
//  * error in case of unforeseen problems
func (rs *ReceiptEquivalenceClass) AddReceipt(receipt *flow.ExecutionReceipt) (uint, error) {
	if receipt.ExecutionResult.ID() != rs.resultID {
		return 0, errors.New("cannot add receipt for different result")
	}

	receiptID := receipt.ID()
	if rs.Has(receiptID) {
		return 0, nil
	}
	rs.receipts[receipt.ID()] = receipt.Meta()
	return 1, nil
}

// AddReceipts adds the receipts to the ReceiptEquivalenceClass (the ones not already stored).
// Returns:
//  * uint: number of receipts added
//  * error in case of unforeseen problems
func (rs *ReceiptEquivalenceClass) AddReceipts(receipts ...*flow.ExecutionReceipt) (uint, error) {
	receiptsAdded := uint(0)
	for i := 0; i < len(receipts); i++ {
		added, err := rs.AddReceipt(receipts[i])
		if err != nil {
			return receiptsAdded, fmt.Errorf("failed to add receipt (%x) to equivalence class: %w", receipts[i].ID(), err)
		}
		receiptsAdded += added
	}
	return receiptsAdded, nil
}

func (rs *ReceiptEquivalenceClass) Has(receiptID flow.Identifier) bool {
	_, found := rs.receipts[receiptID]
	return found
}

// Size returns the number of receipts in the equivalence class (i.e. the number of
// receipts known for that particular result)
func (rs *ReceiptEquivalenceClass) Size() uint {
	return uint(len(rs.receipts))
}

/* Methods implementing LevelledForest's Vertex interface */

func (rs *ReceiptEquivalenceClass) VertexID() flow.Identifier { return rs.resultID }
func (rs *ReceiptEquivalenceClass) Level() uint64             { return rs.blockHeader.Height }
func (rs *ReceiptEquivalenceClass) Parent() (flow.Identifier, uint64) {
	return rs.result.PreviousResultID, rs.blockHeader.Height - 1
}
