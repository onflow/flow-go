package consensus

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
)

// ResultForest is a mempool holding receipts, which is aware of the tree structure
// formed by the results. Internally it utilizes the LevelledForrest.
// Safe for concurrent access.
type ResultForest struct {
	sync.RWMutex
	forest forest.LevelledForest
}

func (rf *ResultForest) Add(block *flow.Header, receipt *flow.ExecutionReceipt) error {
	//sanity check: initial result should be for block
	if block.ID() != receipt.ExecutionResult.BlockID {
		return fmt.Errorf("receipt is for different block")
	}

	vertex, found := rf.forest.GetVertex(receipt.ExecutionResult.ID())
	var receiptsForResult *ReceiptEquivalenceClass
	if !found {
		var err error
		receiptsForResult, err = NewReceiptEquivalenceClass(block, receipt)
		if err != nil {
			return fmt.Errorf("constructing equivalence class for receipt failed: %w", err)
		}
		err = rf.forest.VerifyVertex(receiptsForResult)
		if err != nil {
			return fmt.Errorf("receipt's equivalence class is not a valid vertex for LevelledForest: %w", err)
		}
		rf.forest.AddVertex(receiptsForResult)
	} else {
		receiptsForResult = vertex.(*ReceiptEquivalenceClass)
		receiptsForResult.AddReceipt(receipt)
	}
	return nil
}

func (rf *ResultForest) ReachableReceipts(resultID flow.Identifier, blockFilter, receiptFilter flow.IdentifierFilter) ([]*flow.ExecutionReceipt, error) {
	rf.RLock()
	defer rf.Unlock()

	vertex, found := rf.forest.GetVertex(resultID)
	if !found {
		return nil, fmt.Errorf("unknown result id %x", resultID)
	}

	receipts := make([]*flow.ExecutionReceipt, 0, 10) // we expect just below 10 execution Receipts per call
	rf.reachableReceipts(vertex, blockFilter, receiptFilter, receipts)

	return receipts, nil
}

func (rf *ResultForest) reachableReceipts(vertex forest.Vertex, blockFilter, receiptFilter flow.IdentifierFilter, receipts []*flow.ExecutionReceipt) {
	receiptsForResult := vertex.(*ReceiptEquivalenceClass)
	if !blockFilter(receiptsForResult.blockHeader.ID()) {
		return
	}

	// add all Execution Receipts for result to `receipts` provided they pass the receiptFilter
	for recID, recMeta := range receiptsForResult.receipts {
		if !receiptFilter(recID) {
			continue
		}
		receipt := flow.ExecutionReceiptFromMeta(*recMeta, *receiptsForResult.result)
		receipts = append(receipts, receipt)
	}

	// travers down the tree in a deep-first-search manner
	children := rf.forest.GetChildren(vertex.VertexID())
	for children.HasNext() {
		child := children.NextVertex()
		rf.reachableReceipts(child, blockFilter, receiptFilter, receipts)
	}
}

// PruneUpToLevel prunes all results for all blocks with height UP TO but NOT INCLUDING `limit`
func (rf *ResultForest) PruneUpToHeight(limit uint64) error {
	err := rf.PruneUpToHeight(limit)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}
	return nil
}
