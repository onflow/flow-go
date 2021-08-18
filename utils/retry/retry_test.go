package retry

import (
	"context"
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

	fakeFunc := func(ctx context.Context) error {
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
	ctx := context.Background()
	BackoffExponential(ctx, "TestRetry", fakeFunc, retryMax, retryMilliseconds, zerolog.Nop())
	after := time.Now()

	actualDuration := after.Sub(before).Nanoseconds()
	expectedMinimumDuration := ((retryMilliseconds + retryMilliseconds*2) * time.Millisecond).Nanoseconds()
	assert.True(t, actualDuration >= expectedMinimumDuration)

	// Ensure second that the expected number of calls to the func are made
	assert.True(t, calls == 3)
}

func TestWithTimeout(t *testing.T) {
	const (
		// retry_max is the maximum number of times the func to rey will be executed
		timeoutDuration = 2 * time.Second

		// retry_milliseconds is the number of milliseconds to wait between retries
		retryMilliseconds = 1000
	)

	calls := 0
	ctx, cancel := context.WithCancel(context.Background())
	cancelCtxAfter1Call := func(ctx context.Context) error {
		calls++
		if calls == 1 {
			cancel()
		}
		return fmt.Errorf("fake error")
	}
	WithTimeout(ctx, "TestRetry", cancelCtxAfter1Call, timeoutDuration, retryMilliseconds, zerolog.Nop())

	// ensure retry loop exits when ctx is cancelled and that the expected number of calls to the func are made
	assert.True(t, calls == 1)

	before := time.Now()
	// ensure retry loop exits when timeout is reached
	WithTimeout(context.Background(), "TestRetry", func(ctx context.Context) error {
		return fmt.Errorf("fake error")
	}, timeoutDuration, retryMilliseconds, zerolog.Nop())
	after := time.Now()
	actualDuration := after.Sub(before).Nanoseconds()
	assert.True(t, actualDuration < (3*time.Second).Nanoseconds())
}
