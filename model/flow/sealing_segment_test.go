package flow_test

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// SealingSegmentSuite is the test suite for sealing segment construction and validation.
// Notation:
// B1 - block 1
// R1 - execution result+receipt for block 1
// S1 - seal for block 1
// R* - execution result+receipt for some block before the segment
// S* - seal for some block before the segment
type SealingSegmentSuite struct {
	suite.Suite

	results              map[flow.Identifier]*flow.ExecutionResult
	sealsByBlockID       map[flow.Identifier]*flow.Seal
	protocolStateEntries map[flow.Identifier]*flow.ProtocolStateEntryWrapper
	// bootstrap each test case with a block which is before, and receipt+seal for the block
	priorBlock             *flow.Block
	priorReceipt           *flow.ExecutionReceipt
	priorSeal              *flow.Seal
	defaultProtocolStateID flow.Identifier

	builder *flow.SealingSegmentBuilder
}

func TestSealingSegmentSuite(t *testing.T) {
	suite.Run(t, new(SealingSegmentSuite))
}

// addResult adds the result to the suite mapping.
func (suite *SealingSegmentSuite) addResult(result *flow.ExecutionResult) {
	suite.results[result.ID()] = result
}

// addSeal adds the seal as being the latest w.r.t. the block ID.
func (suite *SealingSegmentSuite) addSeal(blockID flow.Identifier, seal *flow.Seal) {
	suite.sealsByBlockID[blockID] = seal
}

func (suite *SealingSegmentSuite) addProtocolStateEntry(protocolStateID flow.Identifier, entry *flow.ProtocolStateEntryWrapper) {
	suite.protocolStateEntries[protocolStateID] = entry
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

// GetProtocolStateEntry gets a protocol state entry from the map in the suite.
func (suite *SealingSegmentSuite) GetProtocolStateEntry(protocolStateID flow.Identifier) (*flow.ProtocolStateEntryWrapper, error) {
	entry, ok := suite.protocolStateEntries[protocolStateID]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return entry, nil
}

// SetupTest resets maps and creates a new builder for a new test case.
func (suite *SealingSegmentSuite) SetupTest() {
	suite.results = make(map[flow.Identifier]*flow.ExecutionResult)
	suite.sealsByBlockID = make(map[flow.Identifier]*flow.Seal)
	suite.protocolStateEntries = make(map[flow.Identifier]*flow.ProtocolStateEntryWrapper)
	suite.builder = flow.NewSealingSegmentBuilder(suite.GetResult, suite.GetSealByBlockID, suite.GetProtocolStateEntry)

	suite.defaultProtocolStateID = unittest.IdentifierFixture()
	suite.protocolStateEntries[suite.defaultProtocolStateID] = suite.ProtocolStateEntryWrapperFixture()

	priorBlock := suite.BlockFixture()
	priorReceipt, priorSeal := unittest.ReceiptAndSealForBlock(&priorBlock)
	suite.results[priorReceipt.ExecutionResult.ID()] = &priorReceipt.ExecutionResult
	suite.priorBlock = &priorBlock
	suite.priorReceipt = priorReceipt
	suite.priorSeal = priorSeal
}

// BlockFixture returns a Block fixture with the default protocol state ID.
func (suite *SealingSegmentSuite) BlockFixture() flow.Block {
	block := unittest.BlockFixture()
	block.SetPayload(suite.PayloadFixture())
	return block
}

// PayloadFixture returns a Payload fixture with the default protocol state ID.
func (suite *SealingSegmentSuite) PayloadFixture(opts ...func(payload *flow.Payload)) flow.Payload {
	opts = append(opts, unittest.WithProtocolStateID(suite.defaultProtocolStateID))
	return unittest.PayloadFixture(opts...)
}

// BlockWithParentFixture returns a Block fixtures with the default protocol state.
func (suite *SealingSegmentSuite) BlockWithParentFixture(parent *flow.Header) *flow.Block {
	block := unittest.BlockWithParentFixture(parent)
	block.SetPayload(suite.PayloadFixture())
	return block
}

// ProtocolStateEntryWrapperFixture returns a ProtocolStateEntryWrapper.
// For these tests, only the ID matters, so we can just return an empty struct.
func (suite *SealingSegmentSuite) ProtocolStateEntryWrapperFixture() *flow.ProtocolStateEntryWrapper {
	return &flow.ProtocolStateEntryWrapper{}
}

// FirstBlock returns a first block which contains a seal and receipt referencing
// priorBlock (this is the simplest case for a sealing segment).
func (suite *SealingSegmentSuite) FirstBlock() *flow.Block {
	block := suite.BlockFixture()
	block.SetPayload(suite.PayloadFixture(
		unittest.WithSeals(suite.priorSeal),
		unittest.WithReceipts(suite.priorReceipt),
	))
	suite.addSeal(block.ID(), suite.priorSeal)
	return &block
}

// AddBlocks is a short-hand for adding a sequence of blocks, in order.
// No errors are expected.
func (suite *SealingSegmentSuite) AddBlocks(blocks ...*flow.Block) {
	latestSeal := suite.priorSeal
	for _, block := range blocks {
		// before adding block, ensure its latest seal is indexed in suite
		// convention for this test: seals are ordered by height of the sealed block
		for _, seal := range block.Payload.Seals {
			latestSeal = seal
		}
		suite.addSeal(block.ID(), latestSeal)
		err := suite.builder.AddBlock(block)
		require.NoError(suite.T(), err)
	}
}

// Tests the case where a receipt in the segment references a result outside it.
// The result should still be included in the sealing segment.
//
// B1(R*,S*) <- B2(R1) <- B4(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingResultFromReceipt() {

	// B1 contains a receipt (but no result) and seal for a prior block
	block1 := suite.BlockFixture()
	block1.SetPayload(suite.PayloadFixture(unittest.WithReceiptsAndNoResults(suite.priorReceipt), unittest.WithSeals(suite.priorSeal)))

	block2 := suite.BlockWithParentFixture(block1.Header)
	receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(&block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3}, segment.Blocks)
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), suite.priorReceipt.ExecutionResult.ID(), segment.ExecutionResults[0].ID())
}

// Tests the case where the first block contains no seal.
// The latest seal as of the first block should still be included in the segment.
//
// B1 <- B2(R1) <- B3(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingFirstBlockSeal() {

	// B1 contains an empty payload
	block1 := suite.BlockFixture()
	// latest seal as of B1 is priorSeal
	suite.sealsByBlockID[block1.ID()] = suite.priorSeal

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(&block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

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
// B1(S*,R*) <- B2(R1,S**) <- B3(S1)
func (suite *SealingSegmentSuite) TestBuild_MissingResultFromPayloadSeal() {

	block1 := suite.FirstBlock()

	// create a seal referencing some past receipt/block
	pastResult := unittest.ExecutionResultFixture()
	suite.addResult(pastResult)
	pastSeal := unittest.Seal.Fixture()
	pastSeal.ResultID = pastResult.ID()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1), unittest.WithSeals(pastSeal)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{block1, block2, block3}, segment.Blocks)
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), pastResult.ID(), segment.ExecutionResults[0].ID())
}

// Tests the case where the final block in the segment contains both a seal
// for lowest, and a seal for a block above lowest. This should be considered
// an invalid segment.
//
// B1(S*,R*) <- B2 <- B3(R1,R2) <- B4(S1,S2)
func (suite *SealingSegmentSuite) TestBuild_WrongLatestSeal() {

	block1 := suite.FirstBlock()
	block2 := suite.BlockWithParentFixture(block1.Header)

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1, receipt2)))

	block4 := suite.BlockWithParentFixture(block3.Header)
	block4.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1, seal2)))

	suite.AddBlocks(block1, block2, block3, block4)

	_, err := suite.builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentMissingSeal)
}

// Tests the case where the final block in the segment seals multiple
// blocks, but the latest sealed is still lowest, hence it is a valid
// sealing segment.
//
// B1(S*,R*) <- B2(R1) <- B3(S**,S1)
func (suite *SealingSegmentSuite) TestBuild_MultipleFinalBlockSeals() {

	block1 := suite.FirstBlock()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	// create a seal referencing some past receipt/block
	pastResult := unittest.ExecutionResultFixture()
	suite.addResult(pastResult)
	pastSeal := unittest.Seal.Fixture()
	pastSeal.ResultID = pastResult.ID()

	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(pastSeal, seal1)))

	suite.AddBlocks(block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{block1, block2, block3}, segment.Blocks)
	require.Equal(suite.T(), 1, segment.ExecutionResults.Size())
	require.Equal(suite.T(), pastResult.ID(), segment.ExecutionResults[0].ID())
	require.NoError(suite.T(), segment.Validate())
}

// TestBuild_RootSegment tests we can build a valid root sealing segment.
func (suite *SealingSegmentSuite) TestBuild_RootSegment() {

	root, result, seal := unittest.BootstrapFixture(unittest.IdentityListFixture(5, unittest.WithAllRoles()))
	suite.sealsByBlockID[root.ID()] = seal
	suite.addProtocolStateEntry(root.Payload.ProtocolStateID, suite.ProtocolStateEntryWrapperFixture())
	suite.addResult(result)
	err := suite.builder.AddBlock(root)
	require.NoError(suite.T(), err)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{root}, segment.Blocks)
	require.Equal(suite.T(), segment.Highest().ID(), root.ID())
	require.Equal(suite.T(), segment.Sealed().ID(), root.ID())
}

// TestBuild_RootSegmentWrongView tests that we return ErrSegmentInvalidRootView for
// a single-block sealing segment with a block view not equal to 0.
func (suite *SealingSegmentSuite) TestBuild_RootSegmentWrongView() {

	root, result, seal := unittest.BootstrapFixture(
		unittest.IdentityListFixture(5, unittest.WithAllRoles()),
		func(block *flow.Block) {
			block.Header.View = 10 // invalid root block view
		})
	suite.sealsByBlockID[root.ID()] = seal
	suite.addProtocolStateEntry(root.Payload.ProtocolStateID, suite.ProtocolStateEntryWrapperFixture())
	suite.addResult(result)
	err := suite.builder.AddBlock(root)
	require.NoError(suite.T(), err)

	_, err = suite.builder.SealingSegment()
	require.Error(suite.T(), err)
}

// Test the case when the highest block in the segment does not contain seals but
// the first ancestor of the highest block does contain a seal for lowest,
// we return a valid sealing segment.
//
// B1(S*) <- B2(R1) <- B3(S1) <- B4
func (suite *SealingSegmentSuite) TestBuild_HighestContainsNoSeals() {
	block1 := suite.FirstBlock()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1)))

	block4 := suite.BlockWithParentFixture(block3.Header)

	suite.AddBlocks(block1, block2, block3, block4)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{block1, block2, block3, block4}, segment.Blocks)
}

// Test that we should return ErrSegmentMissingSeal if highest block contains
// seals but does not contain seal for lowest, when sealing segment is built.
//
// B1(S*) <- B2(R1) <- B3(S1,R2) <- B4(S2)
func (suite *SealingSegmentSuite) TestBuild_HighestContainsWrongSeal() {
	block1 := suite.FirstBlock()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt2), unittest.WithSeals(seal1)))

	// highest block contains wrong seal - invalid
	block4 := suite.BlockWithParentFixture(block3.Header)
	block4.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal2)))

	suite.AddBlocks(block1, block2, block3, block4)

	_, err := suite.builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentMissingSeal)
}

// Test that we should return ErrSegmentMissingSeal if highest block contains
// no seals and first ancestor with seals does not seal lowest, when sealing
// segment is built
//
// B1(S*) <- B2(R1) <- B3(S1,R2) <- B4(S2) <- B5
func (suite *SealingSegmentSuite) TestBuild_HighestAncestorContainsWrongSeal() {
	block1 := suite.FirstBlock()

	receipt1, seal1 := unittest.ReceiptAndSealForBlock(block1)
	block2 := suite.BlockWithParentFixture(block1.Header)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	receipt2, seal2 := unittest.ReceiptAndSealForBlock(block2)
	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt2), unittest.WithSeals(seal1)))

	// ancestor of highest block contains wrong seal - invalid
	block4 := suite.BlockWithParentFixture(block3.Header)
	block4.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal2)))

	block5 := suite.BlockWithParentFixture(block4.Header)

	suite.AddBlocks(block1, block2, block3, block4, block5)

	_, err := suite.builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentMissingSeal)
}

// TestBuild_ChangingProtocolStateID ...
// B1(R*,S*) <- B2(R1) <- B4(S1,PS2)
func (suite *SealingSegmentSuite) TestBuild_ChangingProtocolStateID_Blocks() {
	block1 := suite.BlockFixture()
	block1.SetPayload(suite.PayloadFixture(unittest.WithReceipts(suite.priorReceipt), unittest.WithSeals(suite.priorSeal)))

	protocolStateID2 := unittest.IdentifierFixture()
	suite.addProtocolStateEntry(protocolStateID2, suite.ProtocolStateEntryWrapperFixture())

	block2 := suite.BlockWithParentFixture(block1.Header)
	receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
	block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt1), unittest.WithProtocolStateID(protocolStateID2)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1), unittest.WithProtocolStateID(protocolStateID2)))

	suite.AddBlocks(&block1, block2, block3)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	require.NoError(suite.T(), segment.Validate())

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3}, segment.Blocks)
	// resulting segment must contain both protocol state IDs
	assert.Len(suite.T(), segment.ProtocolStateEntries, 2)
	_, ok := segment.ProtocolStateEntries[suite.defaultProtocolStateID]
	assert.True(suite.T(), ok)
	_, ok = segment.ProtocolStateEntries[protocolStateID2]
	assert.True(suite.T(), ok)
}

// EB2(PS2) <- EB1 <- B1(S*) <- B2(R1) <- B3(S1)
func (suite *SealingSegmentSuite) TestBuild_ChangingProtocolStateID_ExtraBlocks() {
	block1 := suite.BlockFixture()
	block1.SetPayload(suite.PayloadFixture(unittest.WithReceipts(suite.priorReceipt), unittest.WithSeals(suite.priorSeal)))

	block2 := suite.BlockWithParentFixture(block1.Header)
	receipt1, seal1 := unittest.ReceiptAndSealForBlock(&block1)
	block2.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt1)))

	block3 := suite.BlockWithParentFixture(block2.Header)
	block3.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal1)))

	suite.AddBlocks(&block1, block2, block3)

	// construct two extra blocks that connect to the lowest block and add them to builder
	protocolStateID2 := unittest.IdentifierFixture()
	suite.addProtocolStateEntry(protocolStateID2, suite.ProtocolStateEntryWrapperFixture())

	extraBlock1 := suite.BlockFixture()
	extraBlock1.Header.Height = block1.Header.Height - 1
	extraBlock1.SetPayload(unittest.PayloadFixture(unittest.WithProtocolStateID(protocolStateID2)))
	err := suite.builder.AddExtraBlock(&extraBlock1)
	require.NoError(suite.T(), err)

	extraBlock2 := suite.BlockFixture()
	extraBlock2.Header.Height = extraBlock1.Header.Height - 1
	err = suite.builder.AddExtraBlock(&extraBlock2)
	require.NoError(suite.T(), err)

	segment, err := suite.builder.SealingSegment()
	require.NoError(suite.T(), err)
	err = segment.Validate()
	require.NoError(suite.T(), err)

	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&block1, block2, block3}, segment.Blocks)
	unittest.AssertEqualBlocksLenAndOrder(suite.T(), []*flow.Block{&extraBlock2, &extraBlock1}, segment.ExtraBlocks)
	// resulting segment must contain both protocol state IDs
	assert.Len(suite.T(), segment.ProtocolStateEntries, 2)
	_, ok := segment.ProtocolStateEntries[suite.defaultProtocolStateID]
	assert.True(suite.T(), ok)
	_, ok = segment.ProtocolStateEntries[protocolStateID2]
	assert.True(suite.T(), ok)
}

// Test that we should return ErrSegmentBlocksWrongLen if sealing segment is
// built with no blocks.
func (suite *SealingSegmentSuite) TestBuild_NoBlocks() {
	builder := flow.NewSealingSegmentBuilder(nil, nil, nil)
	_, err := builder.SealingSegment()
	require.ErrorIs(suite.T(), err, flow.ErrSegmentBlocksWrongLen)
}

// should return ErrSegmentInvalidBlockHeight if block has invalid height
func (suite *SealingSegmentSuite) TestAddBlock_InvalidHeight() {

	block1 := suite.FirstBlock()
	// block 2 has an invalid height
	block2 := suite.BlockFixture()
	block2.Header.Height = block1.Header.Height + 2

	err := suite.builder.AddBlock(block1)
	require.NoError(suite.T(), err)

	err = suite.builder.AddBlock(&block2)
	require.ErrorIs(suite.T(), err, flow.ErrSegmentInvalidBlockHeight)
}

// TestAddBlock_StorageError tests that errors in the resource getters bubble up.
func TestAddBlock_StorageError(t *testing.T) {

	t.Run("missing result", func(t *testing.T) {
		// create a receipt to include in the first block, whose result is not in storage
		missingReceipt := unittest.ExecutionReceiptFixture()
		block1 := unittest.BlockFixture()
		sealLookup := func(flow.Identifier) (*flow.Seal, error) { return unittest.Seal.Fixture(), nil }
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, fmt.Errorf("not found") }
		protocolStateEntryLookup := func(flow.Identifier) (*flow.ProtocolStateEntryWrapper, error) {
			return &flow.ProtocolStateEntryWrapper{}, nil
		}
		builder := flow.NewSealingSegmentBuilder(resultLookup, sealLookup, protocolStateEntryLookup)

		block1.SetPayload(unittest.PayloadFixture(
			unittest.WithReceiptsAndNoResults(missingReceipt),
			unittest.WithSeals(unittest.Seal.Fixture(unittest.Seal.WithResult(&missingReceipt.ExecutionResult))),
		))

		err := builder.AddBlock(&block1)
		require.ErrorIs(t, err, flow.ErrSegmentResultLookup)
	})

	// create a first block which contains no seal, and the seal isn't in storage
	t.Run("missing seal", func(t *testing.T) {
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return unittest.ExecutionResultFixture(), nil }
		sealLookup := func(flow.Identifier) (*flow.Seal, error) { return nil, fmt.Errorf("not found") }
		protocolStateEntryLookup := func(flow.Identifier) (*flow.ProtocolStateEntryWrapper, error) {
			return &flow.ProtocolStateEntryWrapper{}, nil
		}
		block1 := unittest.BlockFixture()
		block1.SetPayload(flow.EmptyPayload())
		builder := flow.NewSealingSegmentBuilder(resultLookup, sealLookup, protocolStateEntryLookup)

		err := builder.AddBlock(&block1)
		require.ErrorIs(t, err, flow.ErrSegmentSealLookup)
	})

	t.Run("missing protocol state entry", func(t *testing.T) {
		// TODO(5120)
	})
}

// TestAddExtraBlock tests different scenarios for adding extra blocks, covers happy and unhappy path scenarios.
func (suite *SealingSegmentSuite) TestAddExtraBlock() {
	// populate sealing segment with one block
	firstBlock := suite.FirstBlock()
	firstBlock.Header.Height += 100
	suite.AddBlocks(firstBlock)

	suite.T().Run("empty-segment", func(t *testing.T) {
		builder := flow.NewSealingSegmentBuilder(nil, nil, nil)
		block := suite.BlockFixture()
		err := builder.AddExtraBlock(&block)
		require.Error(t, err)
	})
	suite.T().Run("extra-block-does-not-connect", func(t *testing.T) {
		// adding extra block that doesn't connect to the lowest is an error
		extraBlock := suite.BlockFixture()
		extraBlock.Header.Height = firstBlock.Header.Height + 10 // make sure it doesn't connect by height
		err := suite.builder.AddExtraBlock(&extraBlock)
		require.ErrorIs(t, err, flow.ErrSegmentInvalidBlockHeight)
	})
	suite.T().Run("extra-block-not-continuous", func(t *testing.T) {
		builder := flow.NewSealingSegmentBuilder(suite.GetResult, suite.GetSealByBlockID, suite.GetProtocolStateEntry)
		err := builder.AddBlock(firstBlock)
		require.NoError(t, err)
		extraBlock := suite.BlockFixture()
		extraBlock.Header.Height = firstBlock.Header.Height - 1 // make it connect
		err = builder.AddExtraBlock(&extraBlock)
		require.NoError(t, err)
		extraBlockWithSkip := suite.BlockFixture()
		extraBlockWithSkip.Header.Height = extraBlock.Header.Height - 2 // skip one height
		err = builder.AddExtraBlock(&extraBlockWithSkip)
		require.ErrorIs(t, err, flow.ErrSegmentInvalidBlockHeight)
	})
	suite.T().Run("root-segment-extra-blocks", func(t *testing.T) {
		builder := flow.NewSealingSegmentBuilder(suite.GetResult, suite.GetSealByBlockID, suite.GetProtocolStateEntry)
		err := builder.AddBlock(firstBlock)
		require.NoError(t, err)

		extraBlock := suite.BlockFixture()
		extraBlock.Header.Height = firstBlock.Header.Height - 1
		err = builder.AddExtraBlock(&extraBlock)
		require.NoError(t, err)
		_, err = builder.SealingSegment()
		// root segment cannot have extra blocks
		require.Error(t, err)
	})
	suite.T().Run("happy-path", func(t *testing.T) {
		// add a few blocks with results and seals to form a valid sealing segment
		// B1(S*) <- B2(R1) <- B3(S1)

		receipt, seal := unittest.ReceiptAndSealForBlock(firstBlock)
		blockWithER := suite.BlockWithParentFixture(firstBlock.Header)
		blockWithER.SetPayload(suite.PayloadFixture(unittest.WithReceipts(receipt)))

		// add one more block, with seal to the ER
		highestBlock := suite.BlockWithParentFixture(blockWithER.Header)
		highestBlock.SetPayload(suite.PayloadFixture(unittest.WithSeals(seal)))

		suite.AddBlocks(blockWithER, highestBlock)

		// construct two extra blocks that connect to the lowest block and add them to builder
		// EB2 <- EB1 <- B1(S*) <- B2(R1) <- B3(S1)
		extraBlock := suite.BlockFixture()
		extraBlock.Header.Height = firstBlock.Header.Height - 1
		err := suite.builder.AddExtraBlock(&extraBlock)
		require.NoError(t, err)
		secondExtraBlock := suite.BlockFixture()
		secondExtraBlock.Header.Height = extraBlock.Header.Height - 1
		err = suite.builder.AddExtraBlock(&secondExtraBlock)
		require.NoError(t, err)
		segment, err := suite.builder.SealingSegment()
		require.NoError(t, err)
		err = segment.Validate()
		require.NoError(t, err)
	})
}
