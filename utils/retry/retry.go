package retry

import (
	"time"

	"github.com/rs/zerolog"
)

// BackoffExponential makes maxRetry attempts to executed function f. After the nth attempt,
// we wait retryMilliseconds*2^n ms before retrying. Returns an error after
// maxRetry unsuccessful attempts.
func BackoffExponential(f func() error, maxRetry int, retryMilliseconds int, logger zerolog.Logger) bool {
	wait := time.Duration(retryMilliseconds) * time.Millisecond
	for attempt := 1; attempt <= maxRetry; attempt++ {
		err := f()
		if err != nil {
			logger.Warn().Err(err).Msgf("attempt %d/%d failed", attempt, maxRetry)
			time.Sleep(wait)
			wait <<= 1

			logger.Info().Msgf("%v", wait)
			continue
		}
		return true
	}
	return false
}
