package blocktimer

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/state/protocol"
)

// TestBlockTimestamp_Validate tests that validation accepts valid time and rejects invalid
func TestBlockTimestamp_Validate(t *testing.T) {
	t.Parallel()
	builder, err := NewBlockTimer(10, 1000)
	require.NoError(t, err)
	t.Run("parentTime + minInterval + 1", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.minInterval + 1
		require.NoError(t, builder.Validate(parentTime, blockTime))
	})
	t.Run("parentTime + minInterval", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.minInterval
		require.NoError(t, builder.Validate(parentTime, blockTime))
	})
	t.Run("parentTime + minInterval - 1", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.minInterval - 1
		err := builder.Validate(parentTime, blockTime)
		require.Error(t, err)
		require.True(t, protocol.IsInvalidBlockTimestampError(err))
	})
	t.Run("parentTime + maxInterval - 1", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.maxInterval - 1
		require.NoError(t, builder.Validate(parentTime, blockTime))
	})
	t.Run("parentTime + maxInterval", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.maxInterval
		require.NoError(t, builder.Validate(parentTime, blockTime))
	})
	t.Run("parentTime + maxInterval + 1", func(t *testing.T) {
		parentTime := uint64(time.Now().UnixMilli())
		blockTime := parentTime + builder.maxInterval + 1
		err := builder.Validate(parentTime, blockTime)
		require.Error(t, err)
		require.True(t, protocol.IsInvalidBlockTimestampError(err))
	})
}

// TestBlockTimestamp_Build tests that builder correctly generates new block time
func TestBlockTimestamp_Build(t *testing.T) {
	t.Parallel()
	const minInterval uint64 = 100    // milliseconds
	const maxInterval uint64 = 10_000 // milliseconds
	deltas := []uint64{0, minInterval, maxInterval}

	// this test tries to cover next scenarios in generic way:
	// now = parent - 1
	// now = parent
	// now = parent + 1
	// now = parent + minInterval - 1
	// now = parent + minInterval
	// now = parent + minInterval + 1
	// now = parent + maxInterval - 1
	// now = parent + maxInterval
	// now = parent + maxInterval + 1
	for _, durationDelta := range deltas {
		duration := durationDelta
		t.Run(fmt.Sprintf("duration-delta-%d", durationDelta), func(t *testing.T) {
			builder, err := NewBlockTimer(minInterval, maxInterval)
			require.NoError(t, err)

			parentTime := uint64(time.Now().UnixMilli())

			// now = parentTime + delta + {-1, 0, +1}
			for i := -1; i <= 1; i++ {
				builder.generator = func() uint64 {
					return parentTime + duration + uint64(i)
				}

				blockTime := builder.Build(parentTime)
				require.NoError(t, builder.Validate(parentTime, blockTime))
			}
		})
	}
}
