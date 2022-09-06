package timeout

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

func TestConstructor(t *testing.T) {
	c, err := NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(1200), c.MinReplicaTimeout)
	require.Equal(t, float64(2000), c.MaxReplicaTimeout)
	require.Equal(t, float64(1.5), c.TimeoutIncrease)
	require.Equal(t, uint64(3), c.HappyPathRounds)
	require.Equal(t, float64(1000), c.BlockRateDelayMS)

	// should not allow negative minReplicaTimeout
	c, err = NewConfig(-1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, time.Second)
	require.True(t, model.IsConfigurationError(err))

	// should not allow maxReplicaTimeout < minReplicaTimeout
	c, err = NewConfig(1200*time.Millisecond, 1000*time.Millisecond, 1.5, 3, time.Second)
	require.True(t, model.IsConfigurationError(err))

	// should not allow timeoutIncrease to be 1.0 or smaller
	c, err = NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.0, 3, time.Second)
	require.True(t, model.IsConfigurationError(err))

	// should not allow blockRateDelay to be zero negative
	c, err = NewConfig(1200*time.Millisecond, 2000*time.Millisecond, 1.5, 3, -1*time.Nanosecond)
	require.True(t, model.IsConfigurationError(err))
}

func TestDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()

	require.Equal(t, float64(3000), c.MinReplicaTimeout)
	require.Equal(t, float64(1.2), c.TimeoutIncrease)
	require.Equal(t, uint64(6), c.HappyPathRounds)
	require.Equal(t, float64(0), c.BlockRateDelayMS)
}

func TestStandardTimeoutDecreaseFactor(t *testing.T) {
	timeoutIncreaseFactor := 2.0

	offlineFraction := 1.0 / 3.0
	f := StandardTimeoutDecreaseFactor(offlineFraction, timeoutIncreaseFactor)
	expected := math.Pow(timeoutIncreaseFactor, offlineFraction) * math.Pow(f, 1.0-offlineFraction)
	numericalError := math.Abs(expected - 1.0)
	require.True(t, numericalError < 1e-15)

	offlineFraction = 0.2
	f = StandardTimeoutDecreaseFactor(offlineFraction, timeoutIncreaseFactor)
	expected = math.Pow(timeoutIncreaseFactor, offlineFraction) * math.Pow(f, 1.0-offlineFraction)
	numericalError = math.Abs(expected - 1.0)
	require.True(t, numericalError < 1e-15)
}
