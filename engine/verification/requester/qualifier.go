package requester

import (
	"fmt"
	"time"
)

// RequestQualifierFunc is a function type that on receiving the number of attempts a chunk has been requested with,
// the last time it has been requested, and the duration at which the chunk can be retried after, returns either true or false.
//
// The return value of this function determines whether the chunk request can be dispatched to the network.
type RequestQualifierFunc func(attempts uint64, lastRequested time.Time, retryAfter time.Duration) (bool, string)

// MaxAttemptQualifier only qualifies a chunk request if it has been requested less than the specified number of attempts.
func MaxAttemptQualifier(maxAttempts uint64) RequestQualifierFunc {
	return func(attempts uint64, _ time.Time, _ time.Duration) (bool, string) {
		qualified := attempts < maxAttempts
		if qualified {
			return true, ""
		}
		return false, fmt.Sprintf("max attempts (%d) reached: %d", maxAttempts, attempts)
	}
}

// RetryAfterQualifier only qualifies a chunk request if its retryAfter duration has been elapsed since the last time this
// request has been dispatched.
func RetryAfterQualifier(_ uint64, lastAttempt time.Time, retryAfter time.Duration) (bool, string) {
	nextTry := lastAttempt.Add(retryAfter)
	qualified := nextTry.Before(time.Now())
	if qualified {
		return true, ""
	}
	return false, fmt.Sprintf("retry after not elapsed: next try at %s, in %v",
		nextTry.Format(time.RFC3339), time.Until(nextTry))
}
