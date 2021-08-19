package retry

import (
	"context"
	"time"

	"github.com/rs/zerolog"
)

// BackoffExponential makes maxRetry attempts to executed function f. After the nth attempt,
// we wait retryMilliseconds*2^n ms before retrying. Returns an error after
// maxRetry unsuccessful attempts. It will short circuit if ctx is Done.
func BackoffExponential(ctx context.Context, retryFuncName string, f func(context.Context) error, maxRetry int, retryMilliseconds time.Duration, logger zerolog.Logger) bool {
	wait := retryMilliseconds * time.Millisecond
	for attempt := 1; attempt <= maxRetry; attempt++ {
		err := f(ctx)
		if err != nil {
			logger.Warn().Err(err).Str("retrying_func", retryFuncName).Msgf("attempt %d/%d failed", attempt, maxRetry)

			select {
			case <-time.After(wait):
				wait <<= 1
				continue
			case <-ctx.Done():
				return false
			}
		}
		return true
	}
	return false
}

// WithTimeout attempts to execute function f with a retryMilliseconds sleep in between attempts
// until the function f either succeeds, context is done or timeout has been reached.
func WithTimeout(ctx context.Context, retryFuncName string, f func(context.Context) error, timeoutDuration time.Duration, retryMilliseconds time.Duration, logger zerolog.Logger) bool {
	wait := retryMilliseconds * time.Millisecond
	attempt := 0

	ctxWithTimeOut, cancel := context.WithTimeout(ctx, timeoutDuration)

	defer cancel()

	var err error
	for {
		select {
		case <-time.After(wait):
			err = f(ctxWithTimeOut)
			attempt++
			if err != nil {
				logger.Warn().Err(err).Str("retrying_func", retryFuncName).Msgf("attempt %d failed", attempt)
				continue
			}

			return true
		case <-ctxWithTimeOut.Done():
			return false
		}
	}
}
