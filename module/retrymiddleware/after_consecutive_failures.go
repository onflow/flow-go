package retrymiddleware

import (
	"sync"
	"time"

	"github.com/sethvargo/go-retry"
)

// OnMaxConsecutiveFailures func that gets invoked when max consecutive failures has been reached
type OnMaxConsecutiveFailures func(currentAttempt int)

// AfterConsecutiveFailures returns a retry middleware will invoke the OnMaxConsecutiveFailures callback when ever maxConsecutiveFailures
// has been reached for a retry operation
func AfterConsecutiveFailures(maxConsecutiveFailures int, next retry.Backoff, f OnMaxConsecutiveFailures) retry.Backoff {
	var (
		l       sync.Mutex
		attempt int
	)

	return retry.BackoffFunc(func() (time.Duration, bool) {
		l.Lock()
		defer l.Unlock()
		attempt++

		// if we have reached maxConsecutiveFailures failures update client index
		if attempt%maxConsecutiveFailures == 0 {
			//invoke f with current attempt
			f(attempt)
		}

		val, stop := next.Next()
		if stop {
			return 0, true
		}

		return val, false
	})
}
