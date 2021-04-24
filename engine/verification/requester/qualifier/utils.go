package qualifier

import (
	"math"
	"time"
)

type RequestQualifierFunc func(uint64, time.Time, time.Duration) bool

func UnlimitedAttemptQualifier() RequestQualifierFunc {
	return func(uint64, time.Time, time.Duration) bool {
		return true
	}
}

func MaxAttemptQualifier(maxAttempts uint64) RequestQualifierFunc {
	return func(attempts uint64, _ time.Time, _ time.Duration) bool {
		return attempts < maxAttempts
	}
}

func RetryAfterQualifier() RequestQualifierFunc {
	return func(_ uint64, lastAttempt time.Time, retryAfter time.Duration) bool {
		cutoff := lastAttempt.Add(retryAfter)
		if cutoff.After(time.Now()) {
			return true
		}
		return false
	}
}

type RequestBackoffFunc func(uint64) time.Duration

// ExponentialBackoffWithCutoff is a request backoff factory that generates backoff of value
// interval^attempts - 1. For example, if interval = 2, then it returns a request backoff generator
// for the series of 2^attempt - 1.
//
// The generated backoff is substituted with the given cutoff value for backoff longer than the cutoff.
// This is to make sure that we do not backoff indefinitely.
func ExponentialBackoffWithCutoff(interval time.Duration, cutoff time.Duration) RequestBackoffFunc {
	return func(attempts uint64) time.Duration {
		backoff := time.Duration(math.Pow(float64(interval), float64(attempts)) - 1)
		if backoff > cutoff {
			return cutoff
		}
		return backoff
	}
}
