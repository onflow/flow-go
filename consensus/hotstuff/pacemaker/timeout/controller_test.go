package timeout

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	startRepTimeout        float64 = 120 // Milliseconds
	minRepTimeout          float64 = 100 // Milliseconds
	voteTimeoutFraction    float64 = 0.5 // multiplicative factor
	multiplicativeIncrease float64 = 1.5 // multiplicative factor
	additiveDecrease       float64 = 50  // Milliseconds
)

func initTimeoutController(t *testing.T) *Controller {
	tc, err := NewConfig(
		time.Duration(startRepTimeout*1e6),
		time.Duration(minRepTimeout*1e6),
		voteTimeoutFraction,
		multiplicativeIncrease,
		time.Duration(additiveDecrease*1e6), 0)
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

// Test_TimeoutDecrease verifies that timeout decreases linearly
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
			int64(repTimeout-float64(i)*additiveDecrease),
		)
		assert.Equal(t,
			tc.VoteCollectionTimeout().Milliseconds(),
			int64((repTimeout-float64(i)*additiveDecrease)*voteTimeoutFraction),
		)
	}
}

// Test_MinCutoff verifies that timeout does not decrease below minRepTimeout
func Test_MinCutoff(t *testing.T) {
	tc := initTimeoutController(t)

	tc.OnTimeout()               // replica timeout increases 120 -> 1.5 * 120 = 180
	tc.OnProgressBeforeTimeout() // replica timeout decreases 180 -> 180 - 50 = 130
	tc.OnProgressBeforeTimeout() // replica timeout decreases 130 -> max(130 - 50, 100) = 100

	tc.OnProgressBeforeTimeout()
	assert.Equal(t, tc.ReplicaTimeout().Milliseconds(), int64(minRepTimeout))
	assert.Equal(t, tc.VoteCollectionTimeout().Milliseconds(), int64(minRepTimeout*voteTimeoutFraction))
}

// Test_MinCutoff verifies that timeout does not decrease below minRepTimeout
func Test_MaxCutoff(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(200*float64(time.Millisecond)),
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		voteTimeoutFraction,
		10,
		time.Duration(additiveDecrease*float64(time.Millisecond)),
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

func Test_BlockRateDelay(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(
		time.Duration(200*float64(time.Millisecond)),
		time.Duration(minRepTimeout*float64(time.Millisecond)),
		voteTimeoutFraction,
		10,
		time.Duration(additiveDecrease*float64(time.Millisecond)),
		time.Second)
	if err != nil {
		t.Fail()
	}
	tc := NewController(c)
	assert.Equal(t, time.Second, tc.BlockRateDelay())
}
