package consensus

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestReceiptsForest(t *testing.T) {
	suite.Run(t, new(ReceiptsForestSuite))
}

type ReceiptsForestSuite struct {
	suite.Suite
	Forest *ReceiptsForest
}

func (bs *ReceiptsForestSuite) SetupTest() {
	bs.Forest = NewReceiptsForest()
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
func (bs *ReceiptsForestSuite) createExecutionTree() (map[string]*flow.Block, map[string]*flow.ExecutionReceipt) {
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
	rA10 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["A10"]))
	rA11 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["A11"]), unittest.WithPreviousResult(*rA10))

	rB10 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B10"]))
	rB11_1 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B11"]), unittest.WithPreviousResult(*rB10))
	rB12_1 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B12"]), unittest.WithPreviousResult(*rB11_1))
	rB11_2 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B11"]), unittest.WithPreviousResult(*rB10))
	rB12_2 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["B12"]), unittest.WithPreviousResult(*rB11_2))

	rC11 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C11"]), unittest.WithPreviousResult(*rB10))
	rC12 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C12"]), unittest.WithPreviousResult(*rC11))
	rC13 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["C13"]), unittest.WithPreviousResult(*rC12))

	rD13 := unittest.ExecutionResultFixture(unittest.WithBlock(blocks["D13"]))

	// Make Receipts
	executionReceipts := make(map[string]*flow.ExecutionReceipt)

	executionReceipts["ER[r[A10]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rA10))
	executionReceipts["ER[r[A11]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rA11))

	executionReceipts["ER[r[B10]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB10))
	executionReceipts["ER[r[B11]_1]_1"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB11_1))
	executionReceipts["ER[r[B11]_1]_2"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB11_1))
	executionReceipts["ER[r[B12]_1]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB12_1))

	executionReceipts["ER[r[B11]_2]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB11_2))
	executionReceipts["ER[r[B12]_2]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rB12_2))

	executionReceipts["ER[r[C11]]_1"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rC11))
	executionReceipts["ER[r[C11]]_2"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rC11))

	executionReceipts["ER[r[C13]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rC13))
	executionReceipts["ER[r[D13]]"] = unittest.ExecutionReceiptFixture(unittest.WithResult(rD13))

	return blocks, executionReceipts
}

func (bs *ReceiptsForestSuite) addReceipts2ReceiptsForest(receipts map[string]*flow.ExecutionReceipt, bocks map[string]*flow.Block) {
	blockById := make(map[flow.Identifier]*flow.Block)
	for _, block := range bocks {
		blockById[block.ID()] = block
	}
	for name, rcpt := range receipts {
		block := blockById[rcpt.ExecutionResult.BlockID]
		_, err := bs.Forest.Add(rcpt, block.Header)
		if err != nil {
			bs.FailNow("failed to add receipt '%s'", name)
		}
	}
}

// Receipts that are already included in the fork should be skipped.
func (bs *ReceiptsForestSuite) Test_Initialization() {
	assert.Equal(bs.T(), uint(0), bs.Forest.Size())
	assert.Equal(bs.T(), uint64(0), bs.Forest.LowestHeight())
}

// Receipts that are already included in the fork should be skipped.
func (bs *ReceiptsForestSuite) Test_Add() {
	block := unittest.BlockFixture()
	receipt := unittest.ReceiptForBlockFixture(&block)

	// add should succeed and increase size
	added, err := bs.Forest.Add(receipt, block.Header)
	assert.NoError(bs.T(), err)
	assert.True(bs.T(), added)
	assert.Equal(bs.T(), uint(1), bs.Forest.Size())

	// adding different receipt for same result
	receipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(&receipt.ExecutionResult))
	added, err = bs.Forest.Add(receipt2, block.Header)
	assert.NoError(bs.T(), err)
	assert.True(bs.T(), added)
	assert.Equal(bs.T(), uint(2), bs.Forest.Size())

	// repeated addition should be idempotent
	added, err = bs.Forest.Add(receipt, block.Header)
	assert.NoError(bs.T(), err)
	assert.False(bs.T(), added)
	assert.Equal(bs.T(), uint(2), bs.Forest.Size())
}

// Test_FullTreeSearch verifies that Receipt Forest enumerates all receipts that are
// reachable from the given result
func (bs *ReceiptsForestSuite) Test_FullTreeSearch() {
	blockFilter := func(*flow.Header) bool { return true }
	receiptFilter := func(*flow.ExecutionReceipt) bool { return true }

	blocks, receipts := bs.createExecutionTree()
	bs.addReceipts2ReceiptsForest(receipts, blocks)

	// search Execution Tree starting from result `r[A10]`
	collectedReceipts, err := bs.Forest.ReachableReceipts(receipts["ER[r[A10]]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	bs.Assert().True(reflect.DeepEqual(bs.toSet("ER[r[A10]]", "ER[r[A11]]"), bs.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err = bs.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	expected := bs.toSet(
		"ER[r[B10]]", "ER[r[B11]_1]_1", "ER[r[B11]_1]_2", "ER[r[B12]_1]",
		"ER[r[B11]_2]", "ER[r[B12]_2]",
		"ER[r[C11]]_1", "ER[r[C11]]_2",
	)
	bs.Assert().True(reflect.DeepEqual(expected, bs.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[B11]_2`
	collectedReceipts, err = bs.Forest.ReachableReceipts(receipts["ER[r[B11]_2]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	bs.Assert().True(reflect.DeepEqual(bs.toSet("ER[r[B11]_2]", "ER[r[B12]_2]"), bs.receiptSet(collectedReceipts, receipts)))

	// search Execution Tree starting from result `r[C13]`
	collectedReceipts, err = bs.Forest.ReachableReceipts(receipts["ER[r[C13]]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	bs.Assert().True(reflect.DeepEqual(bs.toSet("ER[r[C13]]"), bs.receiptSet(collectedReceipts, receipts)))
}

// Test_RootBlockExcluded checks that ReceiptsForest does not traverses results for excluded forks.
// Specifically, if the root block is excluded, no result should be returned
func (bs *ReceiptsForestSuite) Test_RootBlockExcluded() {
	blocks, receipts := bs.createExecutionTree()
	bs.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(h *flow.Header) bool {
		return blocks["B10"].ID() != h.ID()
	}
	receiptFilter := func(*flow.ExecutionReceipt) bool { return true }

	// search Execution Tree starting from result `r[B10]`
	collectedReceipts, err := bs.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	assert.Empty(bs.T(), collectedReceipts)
}

// Test_FilterChainForks checks that ReceiptsForest does not traverses results for excluded forks
func (bs *ReceiptsForestSuite) Test_FilterChainForks() {
	blocks, receipts := bs.createExecutionTree()
	bs.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(h *flow.Header) bool {
		return blocks["B11"].ID() != h.ID()
	}
	receiptFilter := func(*flow.ExecutionReceipt) bool { return true }

	// search Execution Tree starting from result `r[B10]`: fork starting from B11 should be excluded
	collectedReceipts, err := bs.Forest.ReachableReceipts(receipts["ER[r[B10]]"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	expected := bs.toSet("ER[r[B10]]", "ER[r[C11]]_1", "ER[r[C11]]_2")
	bs.Assert().True(reflect.DeepEqual(expected, bs.receiptSet(collectedReceipts, receipts)))
}

// Test_ExcludeReceiptsForSealedBlock verifies that, even though we are filtering out the
// receipts for the root result, the tree search still traverses to the derived results
func (bs *ReceiptsForestSuite) Test_ExcludeReceiptsForSealedBlock() {
	blocks, receipts := bs.createExecutionTree()
	bs.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(*flow.Header) bool { return true }
	receiptFilter := func(rcpt *flow.ExecutionReceipt) bool {
		// exclude all receipts for block B11
		return rcpt.ExecutionResult.BlockID != blocks["B11"].ID()
	}

	// search Execution Tree starting from result `r[B11]_1`
	collectedReceipts, err := bs.Forest.ReachableReceipts(receipts["ER[r[B11]_1]_1"].ExecutionResult.ID(), blockFilter, receiptFilter)
	assert.NoError(bs.T(), err)
	bs.Assert().True(reflect.DeepEqual(bs.toSet("ER[r[B12]_1]"), bs.receiptSet(collectedReceipts, receipts)))
}

// Test_UnknownResult checks the behaviour of ReceiptsForest when the search is started on an unknown result
func (bs *ReceiptsForestSuite) Test_UnknownResult() {
	blocks, receipts := bs.createExecutionTree()
	bs.addReceipts2ReceiptsForest(receipts, blocks)

	blockFilter := func(*flow.Header) bool { return true }
	receiptFilter := func(rcpt *flow.ExecutionReceipt) bool { return true }

	// search Execution Tree starting from result random result
	_, err := bs.Forest.ReachableReceipts(unittest.IdentifierFixture(), blockFilter, receiptFilter)
	assert.Error(bs.T(), err)

	// search Execution Tree starting from parent result of "ER[r[D13]]"; While the result is referenced,
	// a receipt committing to this result was never added. Hence the search should error
	_, err = bs.Forest.ReachableReceipts(receipts["ER[r[D13]]"].ExecutionResult.PreviousResultID, blockFilter, receiptFilter)
	assert.Error(bs.T(), err)
}

// Test_ReceiptSorted verifies that receipts are ordered in a
// "parent first" manner
func (bs *ReceiptsForestSuite) Test_ReceiptOrdered() {
	// TODO: implement me
	bs.T().Skip()
}

func makeBlockWithHeight(height uint64) *flow.Block {
	block := unittest.BlockFixture()
	block.Header.Height = height
	return &block
}

func makeChildBlock(parent *flow.Block) *flow.Block {
	block := unittest.BlockWithParentFixture(parent.Header)
	return &block
}

func (bs *ReceiptsForestSuite) receiptSet(selected []*flow.ExecutionReceipt, receipts map[string]*flow.ExecutionReceipt) map[string]struct{} {
	id2Name := make(map[flow.Identifier]string)
	for name, rcpt := range receipts {
		id2Name[rcpt.ID()] = name
	}

	names := make(map[string]struct{})
	for _, r := range selected {
		name, found := id2Name[r.ID()]
		if !found {
			bs.FailNow("unknown execution receipt %x", r.ID())
		}
		names[name] = struct{}{}
	}
	return names
}

func (bs *ReceiptsForestSuite) toSet(receiptNames ...string) map[string]struct{} {
	set := make(map[string]struct{})
	for _, name := range receiptNames {
		set[name] = struct{}{}
	}
	if len(set) != len(receiptNames) {
		bs.FailNow("repeated receipts")
	}
	return set
}
