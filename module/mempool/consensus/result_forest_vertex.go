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

func NewReceiptEquivalenceClass(block *flow.Header, receipts ...*flow.ExecutionReceipt) (*ReceiptEquivalenceClass, error) {
	if len(receipts) == 0 {
		return nil, fmt.Errorf("at least one ExecutionReceipt required for creating ReceiptEquivalenceClass")
	}
	initialReceipt := receipts[0]
	initialResult := &initialReceipt.ExecutionResult

	//sanity check: initial result should be for block
	if block.ID() != initialResult.BlockID {
		return nil, fmt.Errorf("initial result is for different block")
	}

	// construct ReceiptEquivalenceClass only containing initialReceipt
	rcpts := make(map[flow.Identifier]*flow.ExecutionReceiptMeta)
	rcpts[initialReceipt.ID()] = initialReceipt.Meta()
	rs := &ReceiptEquivalenceClass{
		receipts:    rcpts,
		result:      initialResult,
		resultID:    initialResult.ID(),
		blockHeader: block,
	}

	// add remaining receipt
	for i := 1; i < len(receipts); i++ {
		err := rs.AddReceipt(receipts[i])
		if err != nil {
			return nil, err
		}
	}

	return rs, nil
}

func (rs *ReceiptEquivalenceClass) AddReceipts(receipts ...*flow.ExecutionReceipt) error {
	for i := 0; i < len(receipts); i++ {
		err := rs.AddReceipt(receipts[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (rs *ReceiptEquivalenceClass) AddReceipt(receipt *flow.ExecutionReceipt) error {
	if receipt.ExecutionResult.ID() != rs.resultID {
		return errors.New("cannot add receipt for different result")
	}
	rs.receipts[receipt.ID()] = receipt.Meta()
	return nil
}

func (rs *ReceiptEquivalenceClass) VertexID() flow.Identifier { return rs.resultID }
func (rs *ReceiptEquivalenceClass) Level() uint64             { return rs.blockHeader.Height }
func (rs *ReceiptEquivalenceClass) Parent() (flow.Identifier, uint64) {
	return rs.result.PreviousResultID, rs.blockHeader.Height - 1
}
