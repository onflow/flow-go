package retrymiddleware

import (
	"sync"
	"time"

	"github.com/sethvargo/go-retry"
)

// OnMaxConsecutiveFailures func that gets invoked when max consecutive failures has been reached
type OnMaxConsecutiveFailures func(totalAttempts, clientIndex int)

// AfterConsecutiveFailures returns a retry middleware that keeps track of the current number of attempts as well as the current client index.
// When maxConsecutiveFailures has been reached the client index will be incremented or reset to 0 and the middleware will invoke f with the new
// client index.
func AfterConsecutiveFailures(maxConsecutiveFailures, numOfClients int, next retry.Backoff, f OnMaxConsecutiveFailures) retry.Backoff {
	var l sync.Mutex

	var attempt, client int

	// invoke f with our initial client index
	f(attempt, client)

	return retry.BackoffFunc(func() (time.Duration, bool) {
		l.Lock()
		defer l.Unlock()
		attempt++

		// if we have reached maxConsecutiveFailures failures update client index
		if attempt%maxConsecutiveFailures == 0 {
			// check if we have reached our last client index
			if client == numOfClients {
				client = 0
			} else {
				client++
			}

			//invoke f with new client index
			f(attempt, client)
		}

		val, stop := next.Next()
		if stop {
			return 0, true
		}

		return val, false
	})
}
