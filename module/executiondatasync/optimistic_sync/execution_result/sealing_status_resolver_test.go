package execution_result

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealingStatusResolver(t *testing.T) {
	t.Run("IsSealed", func(t *testing.T) {
		block := unittest.BlockFixture()
		blockID := block.ID()

		t.Run("Error getting block header", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			resolver := NewSealingStatusResolver(headers, state)

			headers.On("ByBlockID", blockID).Return(nil, errors.New("error"))

			sealed, err := resolver.IsSealed(blockID)
			assert.Error(t, err)
			assert.False(t, sealed)
		})

		t.Run("No finalized block at height", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			resolver := NewSealingStatusResolver(headers, state)

			headers.On("ByBlockID", blockID).Return(block.ToHeader(), nil)
			headers.On("BlockIDByHeight", block.Height).Return(flow.ZeroID, errors.New("not found"))

			sealed, err := resolver.IsSealed(blockID)
			assert.NoError(t, err)
			assert.False(t, sealed)
		})

		t.Run("Different finalized block at height", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			resolver := NewSealingStatusResolver(headers, state)

			otherBlockID := unittest.BlockFixture().ID()
			headers.On("ByBlockID", blockID).Return(block.ToHeader(), nil)
			headers.On("BlockIDByHeight", block.Height).Return(otherBlockID, nil)

			sealed, err := resolver.IsSealed(blockID)
			assert.NoError(t, err)
			assert.False(t, sealed)
		})

		t.Run("Error getting sealed head", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			snapshot := protocol.NewSnapshot(t)
			resolver := NewSealingStatusResolver(headers, state)

			headers.On("ByBlockID", blockID).Return(block.ToHeader(), nil)
			headers.On("BlockIDByHeight", block.Height).Return(blockID, nil)

			state.On("Sealed").Return(snapshot)
			snapshot.On("Head").Return(nil, errors.New("error"))

			sealed, err := resolver.IsSealed(blockID)
			assert.Error(t, err)
			assert.False(t, sealed)
		})

		t.Run("Block is not sealed (height > sealed height)", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			snapshot := protocol.NewSnapshot(t)
			resolver := NewSealingStatusResolver(headers, state)

			headers.On("ByBlockID", blockID).Return(block.ToHeader(), nil)
			headers.On("BlockIDByHeight", block.Height).Return(blockID, nil)

			sealedHeader := unittest.BlockFixture().ToHeader()
			sealedHeader.Height = block.Height - 1

			state.On("Sealed").Return(snapshot)
			snapshot.On("Head").Return(sealedHeader, nil)

			sealed, err := resolver.IsSealed(blockID)
			assert.NoError(t, err)
			assert.False(t, sealed)
		})

		t.Run("Block is sealed (height <= sealed height)", func(t *testing.T) {
			headers := storagemock.NewHeaders(t)
			state := protocol.NewState(t)
			snapshot := protocol.NewSnapshot(t)
			resolver := NewSealingStatusResolver(headers, state)

			headers.On("ByBlockID", blockID).Return(block.ToHeader(), nil)
			headers.On("BlockIDByHeight", block.Height).Return(blockID, nil)

			sealedHeader := unittest.BlockFixture().ToHeader()
			sealedHeader.Height = block.Height + 1

			state.On("Sealed").Return(snapshot)
			snapshot.On("Head").Return(sealedHeader, nil)

			sealed, err := resolver.IsSealed(blockID)
			assert.NoError(t, err)
			assert.True(t, sealed)
		})
	})
}
