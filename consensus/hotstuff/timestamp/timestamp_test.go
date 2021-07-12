package timestamp

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// TestBlockTimestamp_Build tests that builder correctly generates new block time
func TestBlockTimestamp_Build(t *testing.T) {
	t.Parallel()
	t.Run("within interval", func(t *testing.T) {
		builder := NewBlockTimestamp(100*time.Millisecond, 10*time.Second)
		parentTime := time.Now().UTC()
		// wait a bit
		time.Sleep(time.Millisecond * 200)
		blockTime := builder.Build(parentTime)
		require.True(t, blockTime.After(parentTime.Add(builder.minInterval)))
		require.True(t, blockTime.Before(parentTime.Add(builder.maxInterval)))
	})
	t.Run("before interval", func(t *testing.T) {
		builder := NewBlockTimestamp(100*time.Millisecond, 10*time.Second)
		parentTime := time.Now().UTC()
		blockTime := builder.Build(parentTime)
		require.True(t, blockTime.Equal(parentTime.Add(builder.minInterval)))
	})
	t.Run("after interval", func(t *testing.T) {
		builder := NewBlockTimestamp(100*time.Millisecond, 10*time.Second)
		parentTime := time.Now().UTC()
		// adjust time so generate time will always be smaller than maxInterval
		parentTime = parentTime.Add(-builder.maxInterval)
		blockTime := builder.Build(parentTime)
		require.True(t, blockTime.Equal(parentTime.Add(builder.maxInterval)))
	})
}

func TestBlockTimestamp_Validate(t *testing.T) {
	t.Parallel()
	builder := NewBlockTimestamp(10*time.Millisecond, 1*time.Second)
	t.Run("valid time", func(t *testing.T) {
		parentTime := time.Now().UTC()
		blockTime := parentTime.Add(time.Millisecond * 100)
		require.NoError(t, builder.Validate(parentTime, blockTime))
	})
	t.Run("invalid time", func(t *testing.T) {
		parentTime := time.Now().UTC()
		blockTime := parentTime.Add(time.Millisecond * 1)
		err := builder.Validate(parentTime, blockTime)
		require.Error(t, err)
		require.True(t, model.IsInvalidBlockTimestampError(err))
	})
}
