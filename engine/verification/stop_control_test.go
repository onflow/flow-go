package verification

import (
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestStopControl(t *testing.T) {

	stopHeightA := uint64(21)
	stopHeightB := uint64(37)
	stopHeightC := uint64(2137)

	state := new(protocol.State)
	logger := unittest.Logger()
	processedIndex := uint64(5)
	cp := &MockConsumerProgress{processedIndex, nil}

	t.Run("cannot change stop height after stopping has commenced", func(t *testing.T) {
		sc, err := NewStopControl(logger, state, cp)
		require.NoError(t, err)

		// first update is always successful
		prev, err := sc.SetStopHeight(stopHeightA)
		require.NoError(t, err)
		require.Equal(t, uint64(0), prev)

		require.False(t, sc.HasCommenced())

		// no stopping has started yet, block below stop height
		skip := sc.ShouldSkipChunkAtHeight(stopHeightA - 1)
		require.False(t, skip)

		require.Equal(t, sc.GetStopHeight(), stopHeightA)

		prev, err = sc.SetStopHeight(stopHeightB)
		require.NoError(t, err)
		require.Equal(t, stopHeightA, prev)

		// block at stop height, it should be skipped
		skip = sc.ShouldSkipChunkAtHeight(stopHeightB)
		require.True(t, skip)

		require.True(t, sc.HasCommenced())

		prev, err = sc.SetStopHeight(stopHeightC)
		require.Error(t, err)

		// state did not change
		require.True(t, sc.HasCommenced())
		require.Equal(t, stopHeightB, sc.GetStopHeight())

		// stop height - 1 is the latest we use
		skip = sc.ShouldSkipChunkAtHeight(stopHeightB)
		require.True(t, skip)
	})

	t.Run("can set commenced status during chunk check", func(t *testing.T) {

		blockID := unittest.IdentifierFixture()
		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				BlockID: blockID,
			},
		}

		header := &flow.Header{
			Height: stopHeightA,
		}

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(header, nil)
		state.On("AtBlockID", blockID).Return(snapshot)

		sc, err := NewStopControl(logger, state, cp)
		require.NoError(t, err)

		// no stop has been set, so no skipping
		skip, err := sc.ShouldSkipChunk(chunk)
		require.NoError(t, err)
		require.False(t, skip)

		prev, err := sc.SetStopHeight(stopHeightA)
		require.NoError(t, err)
		require.False(t, sc.HasCommenced())
		require.Equal(t, uint64(0), prev)

		// now we should skip chunks and has commenced state
		skip, err = sc.ShouldSkipChunk(chunk)
		require.NoError(t, err)
		require.True(t, skip)
		require.True(t, sc.HasCommenced())
	})

	t.Run("initializes using ConsumerProgress directly", func(t *testing.T) {

		sc, err := NewStopControl(logger, state, cp)
		require.NoError(t, err)

		// since we cannot read limit directly, we check if stop height fails appropriately for expected value
		_, err = sc.SetStopHeight(processedIndex - 1)
		require.Error(t, err)

		_, err = sc.SetStopHeight(processedIndex)
		require.NoError(t, err)
	})

	t.Run("initializes using sealed height is ConsumerProgress fails", func(t *testing.T) {
		cp := &MockConsumerProgress{processedIndex, fmt.Errorf("failed")}

		header := &flow.Header{
			Height: processedIndex,
		}

		snapshot := new(protocol.Snapshot)
		snapshot.On("Head").Return(header, nil)
		state.On("Sealed").Return(snapshot)

		sc, err := NewStopControl(logger, state, cp)
		require.NoError(t, err)

		// since we cannot read limit directly, we check if stop height fails appropriately for expected value
		_, err = sc.SetStopHeight(processedIndex - 1)
		require.Error(t, err)

		_, err = sc.SetStopHeight(processedIndex)
		require.NoError(t, err)
	})

}

type MockConsumerProgress struct {
	processedIndex uint64
	err            error
}

func (m *MockConsumerProgress) ProcessedIndex() (uint64, error) {
	return m.processedIndex, m.err
}
func (m MockConsumerProgress) InitProcessedIndex(_ uint64) error { panic("implement me") }
func (m MockConsumerProgress) SetProcessedIndex(_ uint64) error  { panic("implement me") }
