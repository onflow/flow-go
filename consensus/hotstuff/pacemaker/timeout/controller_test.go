package timeout

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	minRepTimeout             float64 = 100   // Milliseconds
	maxRepTimeout             float64 = 10000 // Milliseconds
	timeoutAdjustmentFactor   float64 = 1.5   // timeout duration adjustment factor
	happyPathMaxRoundFailures uint64  = 3     // number of failed rounds before increasing timeouts
)

func initTimeoutController(t *testing.T) *Controller {
	tc, err := NewConfig(time.Duration(minRepTimeout*1e6), time.Duration(maxRepTimeout*1e6), timeoutAdjustmentFactor, happyPathMaxRoundFailures, time.Duration(maxRepTimeout*1e6))
	if err != nil {
		t.Fail()
	}
	return NewController(tc)
}

// Test_TimeoutInitialization timeouts are initialized and reported properly
func Test_TimeoutInitialization(t *testing.T) {
	tc := initTimeoutController(t)
	assert.Equal(t, tc.replicaTimeout(), minRepTimeout)

	// verify that initially returned timeout channel is closed and `nil` is returned as `TimerInfo`
	select {
	case <-tc.Channel():
		break
	default:
		assert.Fail(t, "timeout channel did not return")
	}
	tc.Channel()
}

// Test_TimeoutIncrease verifies that timeout increases exponentially
func Test_TimeoutIncrease(t *testing.T) {
	tc := initTimeoutController(t)

	// advance failed rounds beyond `happyPathMaxRoundFailures`;
	for r := uint64(0); r < happyPathMaxRoundFailures; r++ {
		tc.OnTimeout()
	}

	for r := 1; r <= 10; r += 1 {
		tc.OnTimeout()
		assert.Equal(t,
			tc.replicaTimeout(),
			minRepTimeout*math.Pow(timeoutAdjustmentFactor, float64(r)),
		)
	}
}

// Test_TimeoutDecrease verifies that timeout decreases exponentially
func Test_TimeoutDecrease(t *testing.T) {
	tc := initTimeoutController(t)

	// failed rounds counter
	r := uint64(0)

	// advance failed rounds beyond `happyPathMaxRoundFailures`; subsequent progress should reduce timeout again
	for ; r <= happyPathMaxRoundFailures*2; r++ {
		tc.OnTimeout()
	}
	for ; r > happyPathMaxRoundFailures; r-- {
		tc.OnProgressBeforeTimeout()
		assert.Equal(t,
			tc.replicaTimeout(),
			minRepTimeout*math.Pow(timeoutAdjustmentFactor, float64(r-1-happyPathMaxRoundFailures)),
		)
	}
}

// Test_MinCutoff verifies that timeout does not decrease below minRepTimeout
func Test_MinCutoff(t *testing.T) {
	tc := initTimeoutController(t)

	for r := uint64(0); r < happyPathMaxRoundFailures; r++ {
		tc.OnTimeout() // replica timeout doesn't increase since r < happyPathMaxRoundFailures.
	}

	tc.OnTimeout()               // replica timeout increases 100 -> 3/2 * 100 = 150
	tc.OnTimeout()               // replica timeout increases 150 -> 3/2 * 150 = 225
	tc.OnProgressBeforeTimeout() // replica timeout decreases 225 -> 180 * 2/3 = 150
	tc.OnProgressBeforeTimeout() // replica timeout decreases 150 -> 153 * 2/3 = 100
	tc.OnProgressBeforeTimeout() // replica timeout decreases 100 -> 100 * 2/3 = max(66.6, 100) = 100

	tc.OnProgressBeforeTimeout()
	assert.Equal(t, tc.replicaTimeout(), minRepTimeout)
}

// Test_MaxCutoff verifies that timeout does not increase beyond timeout cap
func Test_MaxCutoff(t *testing.T) {
	tc := initTimeoutController(t)

	// we update the following two values here in the test, which is a naive reference implementation
	unboundedReferenceTimeout := minRepTimeout
	r := -1 * int64(happyPathMaxRoundFailures) // only start increasing `unboundedReferenceTimeout` when this becomes positive

	// add timeouts until our `unboundedReferenceTimeout` exceeds the limit
	for {
		tc.OnTimeout()
		if r++; r > 0 {
			unboundedReferenceTimeout *= timeoutAdjustmentFactor
		}
		if unboundedReferenceTimeout > maxRepTimeout {
			assert.True(t, tc.replicaTimeout() <= maxRepTimeout)
			return // end of test
		}
	}
}

// Test_CombinedIncreaseDecreaseDynamics verifies that timeout increases and decreases
// work as expected in combination
func Test_CombinedIncreaseDecreaseDynamics(t *testing.T) {
	increase, decrease := true, false
	testDynamicSequence := func(seq []bool) {
		tc := initTimeoutController(t)
		tc.cfg.HappyPathMaxRoundFailures = 0 // set happy path rounds to zero to simplify calculation
		numberIncreases, numberDecreases := 0, 0
		for _, increase := range seq {
			if increase {
				numberIncreases += 1
				tc.OnTimeout()
			} else {
				numberDecreases += 1
				tc.OnProgressBeforeTimeout()
			}
		}

		expectedRepTimeout := minRepTimeout * math.Pow(timeoutAdjustmentFactor, float64(numberIncreases-numberDecreases))
		numericalError := math.Abs(expectedRepTimeout - tc.replicaTimeout())
		require.LessOrEqual(t, numericalError, 1.0) // at most one millisecond numerical error
	}

	testDynamicSequence([]bool{increase, increase, increase, decrease, decrease, decrease})
	testDynamicSequence([]bool{increase, decrease, increase, decrease, increase, decrease})
	testDynamicSequence([]bool{increase, increase, increase, increase, increase, decrease})
}
