package timeout

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConstructor(t *testing.T) {
	c, err := NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0.73, 1.5, 0.85, time.Second)
	require.NoError(t, err)
	require.Equal(t, float64(2200), c.ReplicaTimeout)
	require.Equal(t, float64(1200), c.MinReplicaTimeout)
	require.Equal(t, float64(0.73), c.VoteAggregationTimeoutFraction)
	require.Equal(t, float64(1.5), c.TimeoutIncrease)
	require.Equal(t, float64(0.85), c.TimeoutDecrease)
	require.Equal(t, float64(1000), c.BlockRateDelayMS)

	// should not allow startReplicaTimeout < minReplicaTimeout
	_, err = NewConfig(800*time.Millisecond, 1200*time.Millisecond, 0.73, 1.5, 0.85, time.Second)
	require.Error(t, err)

	// should not allow negative minReplicaTimeout
	c, err = NewConfig(2200*time.Millisecond, -1200*time.Millisecond, 0.73, 1.5, 0.85, time.Second)
	require.Error(t, err)

	// should not allow voteAggregationTimeoutFraction to be 0 or larger than 1
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0, 1.5, 0.85, time.Second)
	require.Error(t, err)
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 1.00001, 1.5, 0.85, time.Second)
	require.Error(t, err)

	// should not allow timeoutIncrease to be 1.0 or smaller
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0.73, 1.0, 0.85, time.Second)
	require.Error(t, err)

	// should not allow timeoutDecrease to be zero or 1.0
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0.73, 1.5, 0, time.Second)
	require.Error(t, err)
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0.73, 1.5, 1, time.Second)
	require.Error(t, err)

	// should not allow blockRateDelay to be zero negative
	c, err = NewConfig(2200*time.Millisecond, 1200*time.Millisecond, 0.73, 1.5, 0.85, -1*time.Nanosecond)
	require.Error(t, err)
}

func TestDefaultConfig(t *testing.T) {
	c := NewDefaultConfig()

	require.Equal(t, float64(60000), c.ReplicaTimeout)
	require.Equal(t, float64(2000), c.MinReplicaTimeout)
	require.Equal(t, float64(0.5), c.VoteAggregationTimeoutFraction)
	require.Equal(t, float64(2.0), c.TimeoutIncrease)
	require.True(t, math.Abs(1/math.Sqrt(2)-c.TimeoutDecrease) < 1e-15) // need to allow for some numerical error
	require.Equal(t, float64(0), c.BlockRateDelayMS)
}

// TestStandardVoteAggregationTimeoutFraction tests the computation of the standard
// value for `VoteAggregationTimeoutFraction`.
//
func TestStandardVoteAggregationTimeoutFraction(t *testing.T) {
	// test numerical computation for one specific parameter setting
	f := StandardVoteAggregationTimeoutFraction(1200*time.Millisecond, 500*time.Millisecond)
	require.Equal(t, (0.5*1200.0+500.0)/(1200.0+500.0), f)

	// for no blockRateDelay, the standard value should be 0.5
	f = StandardVoteAggregationTimeoutFraction(123456*time.Millisecond, 0)
	require.Equal(t, 0.5, f)

	// for _very_ large blockRateDelay, the standard value should converge to one
	f = StandardVoteAggregationTimeoutFraction(123456*time.Millisecond, 10000*time.Hour)
	require.True(t, f <= 1.0)
	require.True(t, math.Abs(f-1.0) < 1e-5)
}

func TestStandardTimeoutDecreaseFactor(t *testing.T) {
	timeoutIncreaseFactor := 2.0

	offlineFraction := 1.0 / 3.0
	f := StandardTimeoutDecreaseFactor(offlineFraction, timeoutIncreaseFactor)
	expected := math.Pow(timeoutIncreaseFactor, offlineFraction) * math.Pow(f, 1.0-offlineFraction)
	numericalError := math.Abs(expected - 1.0)
	fmt.Println(numericalError)
	require.True(t, numericalError < 1e-15)

	offlineFraction = 0.2
	f = StandardTimeoutDecreaseFactor(offlineFraction, timeoutIncreaseFactor)
	expected = math.Pow(timeoutIncreaseFactor, offlineFraction) * math.Pow(f, 1.0-offlineFraction)
	numericalError = math.Abs(expected - 1.0)
	require.True(t, numericalError < 1e-15)
}
