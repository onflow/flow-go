package timeout

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	minRepTimeout          float64 = 100   // Milliseconds
	maxRepTimeout          float64 = 10000 // Milliseconds
	multiplicativeIncrease float64 = 1.5   // multiplicative factor
	happyPathRounds        uint64  = 3     // number of failed rounds before increasing timeouts
)

func initTimeoutController(t *testing.T) *Controller {
	tc, err := NewConfig(
		time.Duration(minRepTimeout*1e6),
		time.Duration(maxRepTimeout*1e6),
		multiplicativeIncrease,
		happyPathRounds,
		0)
	if err != nil {
		t.Fail()
	}
	return NewController(tc)
}

// Test_TimeoutInitialization timeouts are initialized and reported properly
func Test_TimeoutInitialization(t *testing.T) {
	tc := initTimeoutController(t)
	assert.Equal(t, tc.ReplicaTimeout().Milliseconds(), int64(minRepTimeout))

	// verify that initially returned timeout channel is closed and `nil` is returned as `TimerInfo`
	select {
	case <-tc.Channel():
		break
	default:
		assert.Fail(t, "timeout channel did not return")
	}
	assert.True(t, tc.TimerInfo() == nil)
	tc.Channel()
}

// Test_TimeoutIncrease verifies that timeout increases exponentially
func Test_TimeoutIncrease(t *testing.T) {
	tc := initTimeoutController(t)

	for i := uint64(1); i < 10; i += 1 {
		tc.OnTimeout()
		step := uint64(0)
		if i > happyPathRounds {
			step = 1
		}
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(minRepTimeout*math.Pow(multiplicativeIncrease, float64(i*step))),
		)
	}
}

// Test_TimeoutDecrease verifies that timeout decreases exponentially
func Test_TimeoutDecrease(t *testing.T) {
	tc := initTimeoutController(t)

	// advance rounds to have increased timeout
	for i := uint64(0); i <= happyPathRounds*2; i++ {
		tc.OnTimeout()
	}

	for i := int64(happyPathRounds * 2); i >= 0; i-- {
		tc.OnProgressBeforeTimeout()
		step := int64(0)
		if uint64(i) > happyPathRounds {
			step = 1
		}
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(minRepTimeout*math.Pow(multiplicativeIncrease, float64(i*step))),
		)
	}
}

// Test_MinCutoff verifies that timeout does not decrease below minRepTimeout
func Test_MinCutoff(t *testing.T) {
	tc := initTimeoutController(t)

	tc.OnTimeout()               // replica timeout increases 120 -> 1.5 * 120 = 180
	tc.OnProgressBeforeTimeout() // replica timeout decreases 180 -> 180 * 0.85 = 153
	tc.OnProgressBeforeTimeout() // replica timeout decreases 153 -> 153 * 0.85 = 130.05
	tc.OnProgressBeforeTimeout() // replica timeout decreases 130.05 -> 130.05 * 0.85 = 110.5425
	tc.OnProgressBeforeTimeout() // replica timeout decreases 110.5425 -> max(110.5425 * 0.85, 100) = 100

	tc.OnProgressBeforeTimeout()
	assert.Equal(t, tc.ReplicaTimeout().Milliseconds(), int64(minRepTimeout))
}

// Test_MinCutoff verifies that timeout does not increase beyond timeout cap
func Test_MaxCutoff(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		time.Duration(maxRepTimeout*float64(time.Millisecond)),
		multiplicativeIncrease,
		happyPathRounds,
		0)
	if err != nil {
		t.Fail()
	}
	tc := NewController(c)

	for i := uint64(1); i <= c.HappyPathRounds+uint64(tc.maxExponent)*2; i += 1 {
		tc.OnTimeout() // after 11 iterations we should have reached the max value
		assert.True(t, float64(tc.ReplicaTimeout().Milliseconds()) <= maxRepTimeout)
	}
}

// Test_CombinedIncreaseDecreaseDynamics verifies that timeout increases and decreases
// work as expected in combination
func Test_CombinedIncreaseDecreaseDynamics(t *testing.T) {
	increase, decrease := true, false
	testDynamicSequence := func(seq []bool) {
		tc := initTimeoutController(t)
		tc.cfg.HappyPathRounds = 0 // set happy path rounds to zero to simplify calculation
		var numberIncreases int = 0
		var numberDecreases int = 0
		for _, increase := range seq {
			if increase {
				numberIncreases += 1
				tc.OnTimeout()
			} else {
				numberDecreases += 1
				tc.OnProgressBeforeTimeout()
			}
		}

		expectedRepTimeout := minRepTimeout * math.Pow(multiplicativeIncrease, float64(numberIncreases - numberDecreases))
		numericalError := math.Abs(expectedRepTimeout - float64(tc.ReplicaTimeout().Milliseconds()))
		require.LessOrEqual(t, numericalError, 1.0) // at most one millisecond numerical error
	}

	testDynamicSequence([]bool{increase, increase, increase, decrease, decrease, decrease})
	testDynamicSequence([]bool{increase, decrease, increase, decrease, increase, decrease})
	testDynamicSequence([]bool{increase, increase, increase, increase, increase, decrease})
}

// Test_BlockRateDelay check that correct block rate delay is returned
func Test_BlockRateDelay(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		time.Duration(maxRepTimeout*float64(time.Millisecond)),
		10,
		happyPathRounds,
		time.Second)
	if err != nil {
		t.Fail()
	}
	tc := NewController(c)
	assert.Equal(t, time.Second, tc.BlockRateDelay())
}
