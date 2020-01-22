package timeout

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	startRepTimeout         float64 = 120.0
	minRepTimeout           float64 = 100.0
	voteTimeoutFraction     float64 = 0.5
	multiplicateiveIncrease float64 = 1.5
	additiveDecrease        float64 = 50
)

func initTimeoutController(t *testing.T) *Controller {
	tc, err := NewConfig(startRepTimeout, minRepTimeout, voteTimeoutFraction, multiplicateiveIncrease, additiveDecrease)
	if err != nil {
		t.Fail()
	}
	return NewController(*tc)
}

// Test_TimeoutInitialization timeouts are initialized ands reported properly
func Test_TimeoutInitialization(t *testing.T) {
	tc := initTimeoutController(t)
	assert.Equal(t, tc.ReplicaTimeout().Milliseconds(), int64(startRepTimeout))
	assert.Equal(t, tc.VoteCollectionTimeoutTimeout().Milliseconds(), int64(startRepTimeout*voteTimeoutFraction))
}

// Test_TimeoutIncrease verifies that timeout increases exponentially
func Test_TimeoutIncrease(t *testing.T) {
	tc := initTimeoutController(t)

	for i := 1; i < 10; i += 1 {
		tc.OnTimeout()
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(startRepTimeout*math.Pow(multiplicateiveIncrease, float64(i))),
		)
		assert.Equal(t,
			tc.VoteCollectionTimeoutTimeout().Milliseconds(),
			int64(startRepTimeout*voteTimeoutFraction*math.Pow(multiplicateiveIncrease, float64(i))),
		)
	}
}

// Test_TimeoutDecrease verifies that timeout decreases linearly
func Test_TimeoutDecrease(t *testing.T) {
	tc := initTimeoutController(t)
	tc.OnTimeout()
	tc.OnTimeout()
	tc.OnTimeout()

	repTimeout := startRepTimeout * math.Pow(multiplicateiveIncrease, 3.0)
	for i := 1; i <= 6; i += 1 {
		tc.OnProgressBeforeTimeout()
		assert.Equal(t,
			tc.ReplicaTimeout().Milliseconds(),
			int64(repTimeout-float64(i)*additiveDecrease),
		)
		assert.Equal(t,
			tc.VoteCollectionTimeoutTimeout().Milliseconds(),
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
	assert.Equal(t, tc.VoteCollectionTimeoutTimeout().Milliseconds(), int64(minRepTimeout*voteTimeoutFraction))
}

// Test_MinCutoff verifies that timeout does not decrease below minRepTimeout
func Test_MaxCutoff(t *testing.T) {
	// here we use a different timeout controller with a larger timeoutIncrease to avoid too many iterations
	c, err := NewConfig(200, minRepTimeout, voteTimeoutFraction, 10, additiveDecrease)
	if err != nil {
		t.Fail()
	}
	tc := NewController(*c)

	for i := 1; i <= 50; i += 1 {
		tc.OnTimeout() // after already 7 iterations we should have reached the max value
		assert.True(t, float64(tc.ReplicaTimeout().Milliseconds()) <= timeoutCap)
		assert.True(t, float64(tc.VoteCollectionTimeoutTimeout().Milliseconds()) <= timeoutCap*voteTimeoutFraction)
	}
}
