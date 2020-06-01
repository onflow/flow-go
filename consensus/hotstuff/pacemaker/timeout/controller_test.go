package timeout

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	startRepTimeout        float64 = 120  // Milliseconds
	minRepTimeout          float64 = 100  // Milliseconds
	voteTimeoutFraction    float64 = 0.5  // multiplicative factor
	multiplicativeIncrease float64 = 1.5  // multiplicative factor
	multiplicativeDecrease float64 = 0.85 // multiplicative factor
)

func initTimeoutController(t *testing.T) *Controller {
	tc, err := NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		multiplicativeDecrease,
		0)
	if err != nil {
		t.Fail()
	}
	return NewController(tc)
}

// Test_TimeoutInitialization timeouts are initialized ands reported properly
func Test_TimeoutInitialization(t *testing.T) {
	tc := initTimeoutController(t)
	assert.Equal(t, tc.ReplicaTimeout().Milliseconds(), int64(startRepTimeout))
	assert.Equal(t, tc.VoteCollectionTimeout().Milliseconds(), int64(startRepTimeout*voteTimeoutFraction))

	// verify that returned timeout channel
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

	for i := 1; i < 10; i += 1 {
		tc.OnTimeout()
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(startRepTimeout*math.Pow(multiplicativeIncrease, float64(i))),
		)
		assert.Equal(t,
			tc.VoteCollectionTimeout().Milliseconds(),
			int64(startRepTimeout*voteTimeoutFraction*math.Pow(multiplicativeIncrease, float64(i))),
		)
	}
}

// Test_TimeoutDecrease verifies that timeout decreases exponentially
func Test_TimeoutDecrease(t *testing.T) {
	tc := initTimeoutController(t)
	tc.OnTimeout()
	tc.OnTimeout()
	tc.OnTimeout()

	repTimeout := startRepTimeout * math.Pow(multiplicativeIncrease, 3.0)
	for i := 1; i <= 6; i += 1 {
		tc.OnProgressBeforeTimeout()
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(repTimeout*math.Pow(multiplicativeDecrease, float64(i))),
		)
		assert.Equal(t,
			tc.VoteCollectionTimeout().Milliseconds(),
			int64((repTimeout*math.Pow(multiplicativeDecrease, float64(i)))*voteTimeoutFraction),
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
	assert.Equal(t, tc.VoteCollectionTimeout().Milliseconds(), int64(minRepTimeout*voteTimeoutFraction))
}

// Test_MinCutoff verifies that timeout does not increase beyond timeout cap
func Test_MaxCutoff(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(200*float64(time.Millisecond)),
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		voteTimeoutFraction,
		10,
		multiplicativeDecrease,
		0)
	if err != nil {
		t.Fail()
	}
	tc := NewController(c)

	for i := 1; i <= 50; i += 1 {
		tc.OnTimeout() // after already 7 iterations we should have reached the max value
		assert.True(t, float64(tc.ReplicaTimeout().Milliseconds()) <= timeoutCap)
		assert.True(t, float64(tc.VoteCollectionTimeout().Milliseconds()) <= timeoutCap*voteTimeoutFraction)
	}
}

// Test_CombinedIncreaseDecreaseDynamics verifies that timeout increases and decreases
// work as expected in combination
func Test_CombinedIncreaseDecreaseDynamics(t *testing.T) {
	increase, decrease := true, false
	testDynamicSequence := func(seq []bool) {
		tc := initTimeoutController(t)
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

		expectedRepTimeout := startRepTimeout * math.Pow(multiplicativeIncrease, float64(numberIncreases)) * math.Pow(multiplicativeDecrease, float64(numberDecreases))
		numericalError := math.Abs(expectedRepTimeout - float64(tc.ReplicaTimeout().Milliseconds()))
		require.True(t, numericalError <= 1.0) // at most one millisecond numerical error
		numericalError = math.Abs(expectedRepTimeout*voteTimeoutFraction - float64(tc.VoteCollectionTimeout().Milliseconds()))
		require.True(t, numericalError <= 1.0) // at most one millisecond numerical error
	}

	testDynamicSequence([]bool{increase, increase, increase, decrease, decrease, decrease})
	testDynamicSequence([]bool{increase, decrease, increase, decrease, increase, decrease})
}

func Test_BlockRateDelay(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(200*float64(time.Millisecond)),
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		voteTimeoutFraction,
		10,
		multiplicativeDecrease,
		time.Second)
	if err != nil {
		t.Fail()
	}
	tc := NewController(c)
	assert.Equal(t, time.Second, tc.BlockRateDelay())
}
