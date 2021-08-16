package retry

import (
	"fmt"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func TestBackoffExponential(t *testing.T) {
	const (
		// retry_max is the maximum number of times the func to rey will be executed
		retryMax = 5

		// retry_milliseconds is the number of milliseconds to wait between retries
		retryMilliseconds = 1000

		succeedAfterCalls = 2
	)

	calls := 0

	fakeFunc := func() error {
		calls++
		if calls <= succeedAfterCalls {
			return fmt.Errorf("fake error")
		}
		return nil
	}

	// Ensure first the aggregate time for the func execution exceeds the sum of
	// the expected between-retry wait periods. Since there are 2 failed calls
	// then 1 successful calls, there are 2 wait periods with length m+m*2
	before := time.Now()
	BackoffExponential(fakeFunc, retryMax, retryMilliseconds, zerolog.Nop())
	after := time.Now()

	actualDuration := after.Sub(before).Nanoseconds()
	expectedMinimumDuration := ((retryMilliseconds + retryMilliseconds*2) * time.Millisecond).Nanoseconds()
	assert.True(t, actualDuration >= expectedMinimumDuration)

	// Ensure second that the expected number of calls to the func are made
	assert.True(t, calls == 3)
}
