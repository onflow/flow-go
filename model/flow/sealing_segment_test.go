package flow_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealingSegmentBuilder_AddBlock(t *testing.T) {
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
		require.Error(t, err)
	})
}

func TestSealingSegmentBuilder_SealingSegment(t *testing.T) {
	t.Run("should return ErrSegmentMissingSeal if highest block does not contain seal for lowest", func(t *testing.T) {
		block1 := unittest.BlockFixture()
		receipt1 := unittest.ReceiptForBlockFixture(&block1)
		resultLookup := func(flow.Identifier) (*flow.ExecutionResult, error) { return &receipt1.ExecutionResult, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)

		block1.SetPayload(unittest.PayloadFixture(unittest.WithReceiptsAndNoResults(receipt1)))

		block2 := unittest.BlockWithParentFixture(block1.Header)

		err := builder.AddBlock(&block1)
		require.NoError(t, err)

		err = builder.AddBlock(&block2)
		require.NoError(t, err)

		_, err = builder.SealingSegment()
		require.True(t, errors.Is(err, flow.ErrSegmentMissingSeal))
	})

	t.Run("should ErrSegmentBlocksWrongLen if sealing segment is built with no blocks", func(t *testing.T) {
		resultLookup := func(resultID flow.Identifier) (*flow.ExecutionResult, error) { return nil, nil }
		builder := flow.NewSealingSegmentBuilder(resultLookup)
		_, err := builder.SealingSegment()
		require.True(t, errors.Is(err, flow.ErrSegmentBlocksWrongLen))
	})
}
