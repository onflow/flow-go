package consensus

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/forest"
	"github.com/onflow/flow-go/module/mempool"
)

// ReceiptsForest is a mempool holding receipts, which is aware of the tree structure
// formed by the results. The mempool supports pruning by height: only results
// descending from the latest sealed and finalized result are relevant. Hence, we
// can prune all results for blocks _below_ the latest block with a finalized seal.
// Results of sufficient height for forks that conflict with the finalized fork are
// retained. However, such orphaned forks do not grow anymore and their results
// will be progressively flushed out with increasing sealed-finalized height.
//
// Safe for concurrent access. Internally, the mempool utilizes the LevelledForrest.
// For an in-depth discussion of the core algorithm, see ./Fork-Aware_Mempools.md
type ReceiptsForest struct {
	sync.RWMutex
	forest forest.LevelledForest
	size   uint
}

// NewReceiptsForest instantiates a ReceiptsForest
func NewReceiptsForest() *ReceiptsForest {
	return &ReceiptsForest{
		RWMutex: sync.RWMutex{},
		forest:  *forest.NewLevelledForest(),
		size:    0,
	}
}

// AddResult adds an Execution Result to the Execution Tree (without any receipts), in
// case the result is not already stored in the tree.
// This is useful for crash recovery:
// After recovering from a crash, the mempools are wiped and the sealed results will not
// be stored in the Execution Tree anymore. Adding the result to the tree allows to create
// a vertex in the tree without attaching any Execution Receipts to it.
func (rf *ReceiptsForest) AddResult(result *flow.ExecutionResult, block *flow.Header) error {
	rf.Lock()
	defer rf.Unlock()

	// drop receipts for block heights lower than the lowest height.
	if block.Height < rf.forest.LowestLevel {
		return nil
	}

	// sanity check: initial result should be for block
	if block.ID() != result.BlockID {
		return fmt.Errorf("receipt is for different block")
	}

	_, err := rf.getEquivalenceClass(result, block)
	if err != nil {
		return fmt.Errorf("failed to get equivalence class for result (%x): %w", result.ID(), err)
	}
	return nil
}

// getEquivalenceClass retrieves the Equivalence class for the given result
// or creates a new one and stores it into the levelled forest
func (rf *ReceiptsForest) getEquivalenceClass(result *flow.ExecutionResult, block *flow.Header) (*ReceiptEquivalenceClass, error) {
	vertex, found := rf.forest.GetVertex(result.ID())
	var receiptsForResult *ReceiptEquivalenceClass
	if !found {
		var err error
		receiptsForResult, err = NewReceiptEquivalenceClass(result, block)
		if err != nil {
			return nil, fmt.Errorf("constructing equivalence class for receipt failed: %w", err)
		}
		err = rf.forest.VerifyVertex(receiptsForResult)
		if err != nil {
			return nil, fmt.Errorf("receipt's equivalence class is not a valid vertex for LevelledForest: %w", err)
		}
		rf.forest.AddVertex(receiptsForResult)
		// this Receipt Equivalence class is empty (no receipts); hence we don't need to adjust the mempool size
		return receiptsForResult, nil
	}

	return vertex.(*ReceiptEquivalenceClass), nil
}

// Add the given execution receipt to the memory pool. Requires height
// of the block the receipt is for. We enforce data consistency on an API
// level by using the block header as input.
func (rf *ReceiptsForest) AddReceipt(receipt *flow.ExecutionReceipt, block *flow.Header) (bool, error) {
	rf.Lock()
	defer rf.Unlock()

	// drop receipts for block heights lower than the lowest height.
	if block.Height < rf.forest.LowestLevel {
		return false, nil
	}

	// sanity check: initial result should be for block
	if block.ID() != receipt.ExecutionResult.BlockID {
		return false, fmt.Errorf("receipt is for different block")
	}

	receiptsForResult, err := rf.getEquivalenceClass(&receipt.ExecutionResult, block)
	if err != nil {
		return false, fmt.Errorf("failed to get equivalence class for result (%x): %w", receipt.ExecutionResult.ID(), err)
	}

	added, err := receiptsForResult.AddReceipt(receipt)
	if err != nil {
		return false, fmt.Errorf("failed to add receipt to its equivalence class: %w", err)
	}
	rf.size += added
	return added > 0, nil
}

// ReachableReceipts returns a slice of ExecutionReceipt, whose result
// is computationally reachable from resultID. Context:
//  * Conceptually, the Execution results form a tree, which we refer to as
//    Execution Tree. A fork in the execution can be due to a fork in the main
//    chain. Furthermore, the execution forks if ENs disagree about the result
//    for the same block.
//  * As the ID of an execution result contains the BlockID, which the result
//    for, all Execution Results with the same ID necessarily are for the same
//    block. All Execution Receipts committing to the same result from an
//    equivalence class and can be represented as one vertex in the Execution
//    Tree.
//  * An execution result r1 points (field ExecutionResult.ParentResultID) to
//    its parent result r0 , whose end state was used as the starting state
//    to compute r1. Formally, we have an edge r0 -> r1 in the Execution Tree,
//    if a result r1 is stored in the mempool, whose ParentResultID points to
//    r0.
// ReachableReceipts traverses the Execution Tree from the provided resultID.
// Execution Receipts are traversed in a parent-first manner, meaning that
// a receipt committing to the parent result is traversed first _before_
// the receipt committing to the derived result.
// The algorithm only traverses to results, for which there exists a
// sequence of interim result in the mempool without any gaps.
func (rf *ReceiptsForest) ReachableReceipts(resultID flow.Identifier, blockFilter mempool.BlockFilter, receiptFilter mempool.ReceiptFilter) ([]*flow.ExecutionReceipt, error) {
	rf.RLock()
	defer rf.RUnlock()

	vertex, found := rf.forest.GetVertex(resultID)
	if !found {
		return nil, fmt.Errorf("unknown result id %x", resultID)
	}

	receipts := make([]*flow.ExecutionReceipt, 0, 10) // we expect just below 10 execution Receipts per call
	rf.reachableReceipts(vertex, blockFilter, receiptFilter, &receipts)

	return receipts, nil
}

// reachableReceipts implements a depth-first search over the Execution Tree.
// Entire sub-trees are skipped from search, if their root result is for a block which do _not_ pass the blockFilter
// For each result (vertex in the Execution Tree), which the tree search visits, the known receipts are inspected.
// Receipts that pass the receiptFilter are appended to `receipts` (passes as slice pointer, to allow appending).
func (rf *ReceiptsForest) reachableReceipts(vertex forest.Vertex, blockFilter mempool.BlockFilter, receiptFilter mempool.ReceiptFilter, receipts *[]*flow.ExecutionReceipt) {
	receiptsForResult := vertex.(*ReceiptEquivalenceClass)
	if !blockFilter(receiptsForResult.blockHeader) {
		return
	}

	// add all Execution Receipts for result to `receipts` provided they pass the receiptFilter
	for _, recMeta := range receiptsForResult.receipts {
		receipt := flow.ExecutionReceiptFromMeta(*recMeta, *receiptsForResult.result)
		if !receiptFilter(receipt) {
			continue
		}
		*receipts = append(*receipts, receipt)
	}

	// travers down the tree in a deep-first-search manner
	children := rf.forest.GetChildren(vertex.VertexID())
	for children.HasNext() {
		child := children.NextVertex()
		rf.reachableReceipts(child, blockFilter, receiptFilter, receipts)
	}
}

// PruneUpToHeight prunes all results for all blocks with height up to but
// NOT INCLUDING `newLowestHeight`. Errors if newLowestHeight is lower than
// the previous value (as we cannot recover previously pruned results).
func (rf *ReceiptsForest) PruneUpToHeight(limit uint64) error {
	rf.Lock()
	defer rf.Unlock()

	// count how many receipts are stored in the Execution Tree that will be removed
	numberReceiptsRemoved := uint(0)
	for l := rf.forest.LowestLevel; l < limit; l++ {
		iterator := rf.forest.GetVerticesAtLevel(l)
		for iterator.HasNext() {
			vertex := iterator.NextVertex()
			numberReceiptsRemoved += vertex.(*ReceiptEquivalenceClass).Size()
		}
	}

	// remove vertices and adjust size
	err := rf.forest.PruneUpToLevel(limit)
	if err != nil {
		return fmt.Errorf("pruning Levelled Forest up to height (aka level) %d failed: %w", limit, err)
	}
	rf.size -= numberReceiptsRemoved

	return nil
}

// Size returns the number of receipts stored in the mempool
func (rf *ReceiptsForest) Size() uint {
	return rf.size
}

// LowestHeight returns the lowest height, where results are still stored in the mempool.
func (rf *ReceiptsForest) LowestHeight() uint64 {
	return rf.forest.LowestLevel
}
