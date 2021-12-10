package flow_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// SealingSegmentSuite is the test suite for sealing segment construction and validation.
type SealingSegmentSuite struct {
	suite.Suite

	results        map[flow.Identifier]*flow.ExecutionResult
	sealsByBlockID map[flow.Identifier]*flow.Seal
	// bootstrap each test case with a block which is before, and receipt+seal for the block
	priorBlock   *flow.Block
	priorReceipt *flow.ExecutionReceipt
	priorSeal    *flow.Seal

	builder *flow.SealingSegmentBuilder
}

func TestSealingSegmentSuite(t *testing.T) {
	suite.Run(t, new(SealingSegmentSuite))
}

// addResult adds the result to the suite mapping.
func (suite *SealingSegmentSuite) addResult(result *flow.ExecutionResult) {
	suite.results[result.ID()] = result
}

// GetResult gets a result by ID from the map in the suite.
func (suite *SealingSegmentSuite) GetResult(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	result, ok := suite.results[resultID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return result, nil
}

// GetSealByBlockID gets a seal by block ID from the map in the suite.
func (suite *SealingSegmentSuite) GetSealByBlockID(blockID flow.Identifier) (*flow.Seal, error) {
	seal, ok := suite.sealsByBlockID[blockID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return seal, nil
}

// SetupTest resets maps and creates a new builder for a new test case.
func (suite *SealingSegmentSuite) SetupTest() {
	suite.results = make(map[flow.Identifier]*flow.ExecutionResult)
	suite.sealsByBlockID = make(map[flow.Identifier]*flow.Seal)
	suite.builder = flow.NewSealingSegmentBuilder(suite.GetResult, suite.GetSealByBlockID)

	priorBlock := unittest.BlockFixture()
	priorReceipt, priorSeal := unittest.ReceiptAndSealForBlock(&priorBlock)
	suite.results[priorReceipt.ExecutionResult.ID()] = &priorReceipt.ExecutionResult
	suite.priorBlock = &priorBlock
	suite.priorReceipt = priorReceipt
	suite.priorSeal = priorSeal
}

// FirstBlock returns a first block which contains a seal and receipt referencing
// priorBlock (this is the simplest case for a sealing segment).
func (suite *SealingSegmentSuite) FirstBlock() *flow.Block {
	block := unittest.BlockFixture()
	block.SetPayload(unittest.PayloadFixture(
		unittest.WithSeals(suite.priorSeal),
		unittest.WithReceipts(suite.priorReceipt),
	))
	return &block
}

// AddBlocks is a short-hand for adding a sequence of blocks, in order.
// No errors are expected.
func (suite *SealingSegmentSuite) AddBlocks(blocks ...*flow.Block) {
	for _, block := range blocks {
		err := suite.builder.AddBlock(block)
		require.NoError(suite.T(), err)
	}
}

// Tests the case where a receipt in the segment references a result outside it.
// The result should still be included in the sealing segment.
//
// B1(Receipt_prior,Seal_prior) <- B2(R1) <- B3 <- B4(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingResultFromReceipt() {

	// B1 contains a receipt (but no result) and seal for a prior block
	block1 := unittest.BlockFixture()
	block1.SetPayload(unittest.PayloadFixture(unittest.WithReceiptsAndNoResults(suite.priorReceipt), unittest.WithSeals(suite.priorSeal)))

	block2 := unittest.BlockWithParentFixture(block1.Header)
	receipt, seal := unittest.ReceiptAndSealForBlock(&block1)
	block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt)))

	block3 := unittest.BlockWithParentFixture(block2.Header)

	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal)))

	suite.AddBlocks(&block1, block2, block3, block4)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3, block4}, segment.Blocks)
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), suite.priorReceipt.ExecutionResult.ID(), segment.ExecutionResults[0].ID())
}

// Tests the case where the first block contains no seal.
// The latest seal as of the first block should still be included in the segment.
//
// B1 <- B2(R1) <- B3(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingFirstBlockSeal() {

	// B1 contains an empty payload
	block1 := unittest.BlockFixture()
	// latest seal as of B1 is priorSeal
	suite.sealsByBlockID[block1.ID()] = suite.priorSeal

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := unittest.BlockWithParentFixture(block2.Header)
	block3.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(&block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3}, segment.Blocks)
	// should contain priorSeal as first seal
	require.Equal(suite.T(), suite.priorSeal, segment.FirstSeal)
	// should contain result referenced by first seal
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), suite.priorReceipt.ExecutionResult.ID(), segment.ExecutionResults[0].ID())
}

// Tests the case where a seal contained in a segment block payloads references
// a missing result. The result should still be included in the segment.
//
// B1(S*,R*) <- B2(R1,S**) <- B4(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingResultFromPayloadSeal() {

	block1 := suite.FirstBlock()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := unittest.BlockWithParentFixture(block2.Header)

	block4 := unittest.BlockWithParentFixture(block3.Header)
	block4.SetPayload(unittest.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(&block1, block2, block3, block4)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3, block4}, segment.Blocks)
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), suite.priorReceipt.ExecutionResult.ID(), segment.ExecutionResults[0].ID())
}

// Tests the case where a seal included because the first block contains no seal
// references a missing result. The result should still be included in the segment.
//
// B*(S*) <- B1 <- B2(R1) <- B4(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingResultFromFirstBlockSeal() {
	// TODO
}

// TestBuild_RootSegment tests we can build a valid root sealing segment.
func (suite *SealingSegmentSuite) TestBuild_RootSegment() {

	block1 := unittest.GenesisFixture()
	err := suite.builder.AddBlock(block1)
	require.NoError(suite.T(), err)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{block1}, segment.Blocks)
	require.Equal(suite.T(), segment.Highest().ID(), block1.ID())
	require.Equal(suite.T(), segment.Lowest().ID(), block1.ID())
}

// TestBuild_RootSegmentWrongView tests that we return ErrSegmentInvalidRootView for
// a single-block sealing segment with a block view not equal to 0.
func (suite *SealingSegmentSuite) TestBuild_RootSegmentWrongView() {

	block1 := unittest.BlockFixture()
	block1.Header.View = uint64(1)
	err := suite.builder.AddBlock(&block1)
	require.NoError(suite.T(), err)

	_, err = suite.builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentInvalidRootView)
}

// Test the case when the highest block in the segment does not contain seals but
// the first ancestor of the highest block does contain a seal for lowest,
// we return a valid sealing segment.
//
// B1(S*) <- B2(R1) <- B3(S1) <- B4
func (suite *SealingSegmentSuite) TestBuild_HighestContainsNoSeals() {

}

// Test that we should return ErrSegmentMissingSeal if highest block contains
// seals but does not contain seal for lowest, when sealing segment is built.
//
// B1(S*) <- B2(R1) <- B3(S1,R2) <- B4(S2)
func (suite *SealingSegmentSuite) TestBuild_HighestContainsWrongSeal() {

}

// Test that we should return ErrSegmentMissingSeal if highest block contains
// no seals and first ancestor with seals does not seal lowest, when sealing
// segment is built
//
// B1(S*) <- B2(R1) <- B3(S1,R2) <- B4(S2) <- B5
func (suite *SealingSegmentSuite) TestBuild_HighestAncestorContainsWrongSeal() {

}

// Test that we should return ErrSegmentBlocksWrongLen if sealing segment is
// built with no blocks.
func (suite *SealingSegmentSuite) TestBuild_NoBlocks() {
	builder := flow.NewSealingSegmentBuilder(nil, nil)
	_, err := builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentBlocksWrongLen)
}

// TestAddBlock_StorageError tests that errors in the resource getters bubble up.
func TestAddBlock_StorageError(t *testing.T) {

	t.Run("missing result", func(t *testing.T) {
		// create a receipt to include in the first block, whose result is
		// not in storage
		missingReceipt := unittest.ExecutionReceiptFixture()
		block1 := unittest.BlockFixture()
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, fmt.Errorf("not found") }
		builder := flow.NewSealingSegmentBuilder(resultLookup, nil)

		block1.SetPayload(unittest.PayloadFixture(
			unittest.WithReceiptsAndNoResults(missingReceipt),
			unittest.WithSeals(unittest.Seal.Fixture(unittest.Seal.WithResult(&missingReceipt.ExecutionResult))),
		))

		err := builder.AddBlock(&block1)
		require.ErrorIs(t, err, flow.ErrSegmentResultLookup)
	})

	// create a first block which contains no seal, and the seal isn't in storage
	t.Run("missing seal", func(t *testing.T) {
		sealLookup := func(flow.Identifier) (*flow.Seal, error) { return nil, fmt.Errorf("not found") }
		block1 := unittest.BlockFixture()
		block1.SetPayload(flow.EmptyPayload())
		builder := flow.NewSealingSegmentBuilder(nil, sealLookup)

		err := builder.AddBlock(&block1)
		require.ErrorIs(t, err, flow.ErrSegmentResultLookup) // TODO sentinel type
	})
}

// should return ErrSegmentInvalidBlockHeight if block has invalid height
func TestAddBlock_InvalidHeight(t *testing.T) {

	block1 := unittest.BlockFixture()
	// block 2 has an invalid height
	block2 := unittest.BlockFixture()
	block2.Header.Height = block1.Header.Height + 2

	// ignore all lookups
	resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil }
	sealLookup := func(flow.Identifier) (*flow.Seal, error) { return nil, nil }
	builder := flow.NewSealingSegmentBuilder(resultLookup, sealLookup)

	err := builder.AddBlock(&block1)
	require.NoError(t, err)

	err = builder.AddBlock(&block2)
	require.ErrorIs(t, err, flow.ErrSegmentInvalidBlockHeight)
}
