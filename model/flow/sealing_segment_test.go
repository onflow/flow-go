package flow_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSealingSegmentBuilder_AddBlock checks expected behavior and sanity checks when adding a block to sealing segment
func TestSealingSegmentBuilder_AddBlock(t *testing.T) {

	// this test builds a valid sealing segment and adds 1 missing execution result to SealingSegment.ExecutionResults
	// B1(Rec_1) -> B2
	// expected sealing segment: SealingSegment{Blocks[B1, B2], ExecutionResults[Res_1]}
	t.Run("should add blocks and missing results", func(t *testing.T) {
		block1 := unittest.BlockFixture()
		receipt1 := unittest.ReceiptForBlockFixture(&block1)
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return &receipt1.ExecutionResult, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		block1.SetPayload(unittest.PayloadFixture(unittest.WithReceiptsAndNoResults(receipt1)))

		block2 := unittest.BlockWithParentFixture(block1.Header)
		receipt, seal := unittest.ReceiptAndSealForBlock(&block1)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt), unittest.WithSeals(seal)))

		err := builder.AddBlock(&block1)
		require.NoError(t, err)

		err = builder.AddBlock(&block2)
		require.NoError(t, err)

		segment, err := builder.SealingSegment()
		require.NoError(t, err)

		require.Equal(t, block2.ID(), segment.Highest().ID())
		require.Equal(t, block1.ID(), segment.Lowest().ID())

		_, ok := segment.ExecutionResults.Lookup()[receipt1.ExecutionResult.ID()]
		require.True(t, ok)

		require.Equal(t, 1, segment.ExecutionResults.Size())
	})

	t.Run("should return err if result lookup fails", func(t *testing.T) {
		block1 := unittest.BlockFixture()
		receipt1 := unittest.ReceiptForBlockFixture(&block1)
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, fmt.Errorf("not found") }
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		block1.SetPayload(unittest.PayloadFixture(unittest.WithReceiptsAndNoResults(receipt1)))

		err := builder.AddBlock(&block1)
		require.ErrorIs(t, err, flow.ErrSegmentResultLookup)
	})

	t.Run("should return ErrSegmentInvalidBlockHeight if block has invalid height", func(t *testing.T) {
		block1 := unittest.BlockFixture()
		block2 := unittest.BlockFixture()
		block3 := unittest.BlockWithParentFixture(block2.Header)
 		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		err := builder.AddBlock(&block1)
		require.NoError(t, err)

		err = builder.AddBlock(&block3)
		require.ErrorIs(t, err, flow.ErrSegmentInvalidBlockHeight)
	})
}

// TestSealingSegmentBuilder_SealingSegment checks behavior and sanity checks when building sealing segment
func TestSealingSegmentBuilder_SealingSegment(t *testing.T) {
	t.Run("should return valid sealing segment", func(t *testing.T) {
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil}
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		block1 := unittest.BlockFixture()
		block2 := unittest.BlockWithParentFixture(block1.Header)
		receipt, seal := unittest.ReceiptAndSealForBlock(&block1)
		block2.SetPayload(unittest.PayloadFixture(unittest.WithReceipts(receipt), unittest.WithSeals(seal)))

		err := builder.AddBlock(&block1)
		require.NoError(t, err)
		err = builder.AddBlock(&block2)
		require.NoError(t, err)

		segment, err := builder.SealingSegment()
		require.NoError(t, err)

		require.Equal(t, segment.Highest().ID(), block2.ID())
		require.Equal(t, segment.Lowest().ID(), block1.ID())
	})

	t.Run("should return ErrSegmentMissingSeal if highest block does not contain seal for lowest", func(t *testing.T) {
		block1 := unittest.BlockFixture()
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		block2 := unittest.BlockWithParentFixture(block1.Header)

		err := builder.AddBlock(&block1)
		require.NoError(t, err)

		err = builder.AddBlock(&block2)
		require.NoError(t, err)

		_, err = builder.SealingSegment()
		require.True(t, errors.Is(err, flow.ErrSegmentMissingSeal))
	})

	t.Run("should return ErrSegmentBlocksWrongLen if sealing segment is built with no blocks", func(t *testing.T) {
		resultLookup := func(resultID flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)
		_, err := builder.SealingSegment()
		require.True(t, errors.Is(err, flow.ErrSegmentBlocksWrongLen))
	})
}
