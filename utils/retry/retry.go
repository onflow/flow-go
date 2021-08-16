package retry

import (
	"time"

	"github.com/rs/zerolog"
)

// BackoffExponential makes maxRetry attempts to executed function f. After the nth attempt,
// we wait retryMilliseconds*2^n ms before retrying. Returns an error after
// maxRetry unsuccessful attempts.
func BackoffExponential(f func() error, maxRetry int, retryMilliseconds int, logger zerolog.Logger) bool {
	for attempt := 1; attempt <= maxRetry; attempt++ {
		err := f()
		if err != nil {
			logger.Warn().Err(err).Msgf("attempt %d/%d failed", attempt, maxRetry)
			wait := time.Duration(retryMilliseconds<<(attempt-1)) * time.Millisecond
			time.Sleep(wait)
			continue
		}
		return true
	}
	return false
}
