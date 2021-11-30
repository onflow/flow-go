package consensus

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/module/mempool"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptsForest(t *testing.T) {
	suite.Run(t, new(ExecutionTreeTestSuite))
}

type ExecutionTreeTestSuite struct {
	suite.Suite
	Forest *ExecutionTree
}

func (et *ExecutionTreeTestSuite) SetupTest() {
	et.Forest = NewExecutionTree()
}

// addReceiptForest creates an Execution Tree for testing purposes and stores it in the ResultForest.
// Nomenclature:
//  * `r[A]` Execution Result for block A
//     if we have multiple _different_ results for block A,
//     we denote them as `r[A]_1`, `r[A]_2` ...
//  * `ER[r]` Execution Receipt committing to result `r`
//     if we have multiple _different_ receipts committing to the same result `r`,
//     we denote them as `ER[r]_1`, `ER[r]_2` ...
//  * Multiple Execution Receipts for the same result form an Equivalence Class, e.g. {`ER[r]_1`, `ER[r]_2`, ...}
//    For brevity, we denote the Equivalence Class as `r{ER_1, ER_2, ...}`
//
// We consider the following forest of blocks where the number indicates the block height:
//   : <- A10 <- A11
//   :
//   : <- B10 <- B11 <- B12
//   :       ^-- C11 <- C12 <- C13
//   :
//   :                      ?<- D13
//   pruned height
//
// We construct the following Execution Tree:
//   :
//   : <- r[A10]{ER} <- r[A11]{ER}
//   :
//   : <- r[B10]{ER} <- r[B11]_1 {ER_1, ER_2} <- r[B12]_1 {ER}
//   :              ^-- r[B11]_2 {ER}         <- r[B12]_2 {ER}
//   :             ^
//   :             └-- r[C11] {ER_1, ER_2}    < . . .  ? ? ? ? . .   <- r[C13] {ER}
//   :
//   :                                                       ? ? ? ? <- r[C13] {ER}
//   pruned height
func (et *ExecutionTreeTestSuite) createExecutionTree() (map[string]*flow.Block, map[string]*flow.ExecutionResult, map[string]*flow.ExecutionReceipt) {
	// Make blocks
	blocks := make(map[string]*flow.Block)

	blocks["A10"] = makeBlockWithHeight(10)
	blocks["A11"] = makeChildBlock(blocks["A10"])

	blocks["B10"] = makeBlockWithHeight(10)
	blocks["B11"] = makeChildBlock(blocks["B10"])
	blocks["B12"] = makeChildBlock(blocks["B11"])

	blocks["C11"] = makeChildBlock(blocks["B10"])
	blocks["C12"] = makeChildBlock(blocks["C11"])
	blocks["C13"] = makeChildBlock(blocks["C12"])

	blocks["D13"] = makeBlockWithHeight(13)

	// Make Results
	results := make(map[string]*flow.ExecutionResult)
	results["r[A10]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["A10"]))
	results["r[A11]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["A11"]), unittest.WithPreviousResult(*results["r[A10]"]))

	results["r[B10]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B10"]))
	results["r[B11_1]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B11"]), unittest.WithPreviousResult(*results["r[B10]"]))
	results["r[B12_1]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B12"]), unittest.WithPreviousResult(*results["r[B11_1]"]))
	results["r[B11_2]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B11"]), unittest.WithPreviousResult(*results["r[B10]"]))
	results["r[B12_2]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B12"]), unittest.WithPreviousResult(*results["r[B11_2]"]))

	results["r[C11]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C11"]), unittest.WithPreviousResult(*results["r[B10]"]))
	results["r[C12]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C12"]), unittest.WithPreviousResult(*results["r[C11]"]))
	results["r[C13]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C13"]), unittest.WithPreviousResult(*results["r[C12]"]))

	results["r[D13]"] = unittest.ExecutionResultFixture(unittest.WithBlock(blocks["D13"]))

	// Make Receipts
	receipts := make(map[string]*flow.ExecutionReceipt)

	receipts["ER[r[A10]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[A10]"]))
	receipts["ER[r[A11]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[A11]"]))

	receipts["ER[r[B10]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B10]"]))
	receipts["ER[r[B11]_1]_1"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B11_1]"]))
	receipts["ER[r[B11]_1]_2"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B11_1]"]))
	receipts["ER[r[B12]_1]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B12_1]"]))

	receipts["ER[r[B11]_2]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B11_2]"]))
	receipts["ER[r[B12]_2]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[B12_2]"]))

	receipts["ER[r[C11]]_1"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[C11]"]))
	receipts["ER[r[C11]]_2"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[C11]"]))

	receipts["ER[r[C13]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[C13]"]))
	receipts["ER[r[D13]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(results["r[D13]"]))

	return blocks, results, receipts
}

func (et *ExecutionTreeTestSuite) addReceipts2ReceiptsForest(receipts map[string]*flow.ExecutionReceipt, blocks map[string]*flow.Block) {
	blockById := make(map[flow.Identifier]*flow.Block)
	for _, block := range blocks {
		blockById[block.ID()] = block
	}
	for name, rcpt := range receipts {
		block := blockById[rcpt.ExecutionResult.BlockID]
		_, err := et.Forest.AddReceipt(rcpt, block.Header)
		if err != nil {
			et.FailNow("failed to add receipt '%s'", name)
		}
	}
}

// Receipts that are already included in the fork should be skipped.
func (et *ExecutionTreeTestSuite) Test_Initialization() {
	assert.Equal(et.T(), uint(0), et.Forest.Size())
	assert.Equal(et.T(), uint64(0), et.Forest.LowestHeight())
}

// Test_AddReceipt checks the Forest's AddReceipt method.
// Receipts that are already included in the fork should be skipped.
func (et *ExecutionTreeTestSuite) Test_AddReceipt() {
	block := unittest.BlockFixture()
	receipt := unittest.ReceiptForBlockFixture(&block)

	// add should succeed and increase size
	added, err := et.Forest.AddReceipt(receipt, block.Header)
	assert.NoError(et.T(), err)
	assert.True(et.T(), added)
	assert.Equal(et.T(), uint(1), et.Forest.Size())

	// adding different receipt for same result
	receipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(&receipt.ExecutionResult))
	added, err = et.Forest.AddReceipt(receipt2, block.Header)
	assert.NoError(et.T(), err)
	assert.True(et.T(), added)
	assert.Equal(et.T(), uint(2), et.Forest.Size())

	// repeated addition should be idempotent
	added, err = et.Forest.AddReceipt(receipt, block.Header)
	assert.NoError(et.T(), err)
	assert.False(et.T(), added)
	assert.Equal(et.T(), uint(2), et.Forest.Size())
}

// Test_AddResult_Detached verifies that vertices can be added to the Execution Tree without requiring
// an Execution Receipt. Here, we add a result for a completely detached block. Starting a tree search
// from this result should not yield any receipts.
func (et *ExecutionTreeTestSuite) Test_AddResult_Detached() {
	miscBlock := makeBlockWithHeight(101)
	miscResult := unittest.ExecutionResultFixture(unittest.WithBlock(miscBlock))

	err := et.Forest.AddResult(miscResult, miscBlock.Header)
	assert.NoError(et.T(), err)
	collectedReceipts, err := et.Forest.ReachableReceipts(miscResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	et.Assert().Empty(collectedReceipts)
}

// Test_AddResult_Bridge verifies that vertices can be added to the Execution Tree without requiring
// an Execution Receipt. Here, we add the result r[C12], which closes the gap between r[C11] and r[C13].
// Hence, the tree search should reach r[C13].
func (et *ExecutionTreeTestSuite) Test_AddResult_Bridge() {
	blocks, results, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	// restrict traversal to B10 <- C11 <- C12 <- C13
	blockFilter := func(h *flow.Header) bool {
		for _, blockName := range []string{"B10", "C11", "C12", "C13"} {
			if blocks[blockName].ID() == h.ID() {
				return true
			}
		}
		return false
	}

	// before we add result r[C12], tree search should not be able to reach r[C13]
	collectedReceipts, err := et.Forest.ReachableReceipts(results["r[B10]"].ID(), blockFilter, anyReceipt())
	assert.NoError(et.T(), err)
	expected := et.toSet("ER[r[B10]]", "ER[r[C11]]_1", "ER[r[C11]]_2")
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))

	// after we added r[C12], tree search should reach r[C13] and hence include the corresponding receipt ER[r[C13]]
	err = et.Forest.AddResult(results["r[C12]"], blocks["C12"].Header)
	assert.NoError(et.T(), err)
	collectedReceipts, err = et.Forest.ReachableReceipts(results["r[B10]"].ID(), blockFilter, anyReceipt())
	assert.NoError(et.T(), err)
	expected = et.toSet("ER[r[B10]]", "ER[r[C11]]_1", "ER[r[C11]]_2", "ER[r[C13]]")
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))
}

// Test_FullTreeSearch verifies that Receipt Forest enumerates all receipts that are
// reachable from the given result
func (et *ExecutionTreeTestSuite) Test_FullTreeSearch() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	// search Execution Tree starting from result `r[A10]`
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[A10]]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	et.Assert().True(reflect.DeepEqual(et.toSet("ER[r[A10]]", "ER[r[A11]]"), et.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err = et.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	expected := et.toSet(
		"ER[r[B10]]", "ER[r[B11]_1]_1", "ER[r[B11]_1]_2", "ER[r[B12]_1]",
		"ER[r[B11]_2]", "ER[r[B12]_2]",
		"ER[r[C11]]_1", "ER[r[C11]]_2",
	)
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[B11]_2`
	collectedReceipts, err = et.Forest.ReachableReceipts(receipts["ER[r[B11]_2]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	et.Assert().True(reflect.DeepEqual(et.toSet("ER[r[B11]_2]", "ER[r[B12]_2]"), et.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[C13]`
	collectedReceipts, err = et.Forest.ReachableReceipts(receipts["ER[r[C13]]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	et.Assert().True(reflect.DeepEqual(et.toSet("ER[r[C13]]"), et.receiptSet(collectedReceipts, receipts)))
}

// Test_ReceiptSorted verifies that receipts are ordered in a "parent first" manner
func (et *ExecutionTreeTestSuite) Test_ReceiptOrdered() {
	blocks, results, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)

	// first receipt must be for `r[B10]`
	id := collectedReceipts[0].ExecutionResult.ID()
	et.Assert().Equal(results["r[B10]"].ID(), id)
	// for all subsequent receipts, a receipt committing to the parent result must have been listed before
	knownResults := make(map[flow.Identifier]struct{})
	knownResults[id] = struct{}{}
	for _, rcpt := range collectedReceipts[1:] {
		_, found := knownResults[rcpt.ExecutionResult.PreviousResultID]
		et.Assert().True(found)
		knownResults[rcpt.ExecutionResult.ID()] = struct{}{}
	}
}

// Test_FilterReceipts checks that ExecutionTree does filter Receipts as directed by the ReceiptFilter.
func (et *ExecutionTreeTestSuite) Test_FilterReceipts() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	receiptFilter := func(rcpt *flow.ExecutionReceipt) bool {
		for _, receiptName := range []string{"ER[r[B10]]", "ER[r[B11]_1]_2", "ER[r[B12]_2]", "ER[r[C11]]_1"} {
			if receipts[receiptName].ID() == rcpt.ID() {
				return false
			}
		}
		return true
	}

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), anyBlock(), receiptFilter)
	assert.NoError(et.T(), err)
	expected := et.toSet("ER[r[B11]_1]_1", "ER[r[B12]_1]", "ER[r[B11]_2]", "ER[r[C11]]_2")
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))
}

// Test_RootBlockExcluded checks that ExecutionTree does not traverses results for excluded forks.
// In this specific test, we set the root results' block to be excluded. Therefore, the
// tree search should stop immediately and no result should be returned.
func (et *ExecutionTreeTestSuite) Test_RootBlockExcluded() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(h *flow.Header) bool {
		return blocks["B10"].ID() != h.ID()
	}

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), blockFilter, anyReceipt())
	assert.NoError(et.T(), err)
	assert.Empty(et.T(), collectedReceipts)
}

// Test_FilterChainForks checks that ExecutionTree does not traverses results for excluded forks
func (et *ExecutionTreeTestSuite) Test_FilterChainForks() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(h *flow.Header) bool {
		return blocks["B11"].ID() != h.ID()
	}

	// search Execution Tree starting from result `r[B10]`: fork starting from B11 should be excluded
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), blockFilter, anyReceipt())
	assert.NoError(et.T(), err)
	expected := et.toSet("ER[r[B10]]", "ER[r[C11]]_1", "ER[r[C11]]_2")
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))
}

// Test_ExcludeReceiptsForSealedBlock verifies that, even though we are filtering out the
// receipts for the root result, the tree search still traverses to the derived results
func (et *ExecutionTreeTestSuite) Test_ExcludeReceiptsForSealedBlock() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	receiptFilter := func(rcpt *flow.ExecutionReceipt) bool {
		// exclude all receipts for block B11
		return rcpt.ExecutionResult.BlockID != blocks["B11"].ID()
	}

	// search Execution Tree starting from result `r[B11]_1`
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B11]_1]_1"].ExecutionResult.ID(), anyBlock(), receiptFilter)
	assert.NoError(et.T(), err)
	et.Assert().True(reflect.DeepEqual(et.toSet("ER[r[B12]_1]"), et.receiptSet(collectedReceipts, receipts)))
}

// Test_UnknownResult checks the behaviour of ExecutionTree when the search is started on an unknown result
func (et *ExecutionTreeTestSuite) Test_UnknownResult() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	// search Execution Tree starting from result random result
	_, err := et.Forest.ReachableReceipts(unittest.IdentifierFixture(), anyBlock(), anyReceipt())
	assert.Error(et.T(), err)

	// search Execution Tree starting from parent result of "ER[r[D13]]"; While the result is referenced,
	// a receipt committing to this result was never added. Hence the search should error
	_, err = et.Forest.ReachableReceipts(receipts["ER[r[D13]]"].ExecutionResult.PreviousResultID, anyBlock(), anyReceipt())
	assert.Error(et.T(), err)
}

// Receipts that are already included in the fork should be skipped.
func (et *ExecutionTreeTestSuite) Test_Prune() {
	blocks, _, receipts := et.createExecutionTree()
	et.addReceipts2ReceiptsForest(receipts, blocks)

	assert.Equal(et.T(), uint(12), et.Forest.Size())
	assert.Equal(et.T(), uint64(0), et.Forest.LowestHeight())

	// prunes all receipts for blocks with height _smaller_ than 12
	err := et.Forest.PruneUpToHeight(12)
	assert.NoError(et.T(), err)
	assert.Equal(et.T(), uint(4), et.Forest.Size())

	// now, searching results from r[B11] should fail as the receipts were pruned
	_, err = et.Forest.ReachableReceipts(receipts["ER[r[B11]_1]_2"].ExecutionResult.PreviousResultID, anyBlock(), anyReceipt())
	assert.Error(et.T(), err)

	// now, searching results from r[B12] should fail as the receipts were pruned
	collectedReceipts, err := et.Forest.ReachableReceipts(receipts["ER[r[B12]_1]"].ExecutionResult.ID(), anyBlock(), anyReceipt())
	assert.NoError(et.T(), err)
	expected := et.toSet("ER[r[B12]_1]")
	et.Assert().True(reflect.DeepEqual(expected, et.receiptSet(collectedReceipts, receipts)))
}

func anyBlock() mempool.BlockFilter {
	return func(*flow.Header) bool { return true }
}

func anyReceipt() mempool.ReceiptFilter {
	return func(*flow.ExecutionReceipt) bool { return true }
}

func makeBlockWithHeight(height uint64) *flow.Block {
	block := unittest.BlockFixture()
	block.Header.Height = height
	return &block
}

func makeChildBlock(parent *flow.Block) *flow.Block {
	return unittest.BlockWithParentFixture(parent.Header)
}

func (et *ExecutionTreeTestSuite) receiptSet(selected []*flow.ExecutionReceipt, receipts map[string]*flow.ExecutionReceipt) map[string]struct{} {
	id2Name := make(map[flow.Identifier]string)
	for name, rcpt := range receipts {
		id2Name[rcpt.ID()] = name
	}

	names := make(map[string]struct{})
	for _, r := range selected {
		name, found := id2Name[r.ID()]
		if !found {
			et.FailNow("unknown execution receipt %x", r.ID())
		}
		names[name] = struct{}{}
	}
	return names
}

func (et *ExecutionTreeTestSuite) toSet(receiptNames ...string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, name := range receiptNames {
		set[name] = struct{}{}
	}
	if len(set) != len(receiptNames) {
		et.FailNow("repeated receipts")
	}
	return set
}
