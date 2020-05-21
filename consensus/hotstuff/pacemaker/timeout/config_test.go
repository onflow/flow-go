package timeout

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBlockRateDelay(t *testing.T) {
	c, err := NewConfig(1200*time.Millisecond, 1200*time.Millisecond, 0.5, 1.5, 800*time.Millisecond, time.Second)
	require.NoError(t, err)

	require.Equal(t, float64(1000), c.BlockRateDelayMS)
	require.Equal(t, (float64(600+1000))/(float64(1200+1000)), c.VoteAggregationTimeoutFraction)
}
