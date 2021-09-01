package requester

import (
	"math"
	"time"
)

type Config struct {
	BatchInterval   time.Duration // minimum interval between requests
	BatchThreshold  uint          // maximum batch size for one request
	RetryInitial    time.Duration // interval after which we retry request for an entity
	RetryFunction   RetryFunc     // function determining growth of retry interval
	RetryMaximum    time.Duration // maximum interval for retrying request for an entity
	RetryAttempts   uint          // maximum amount of request attempts per entity
	ValidateStaking bool          // should staking of target/origin be checked
}

type RetryFunc func(time.Duration) time.Duration

func RetryConstant() RetryFunc {
	return func(interval time.Duration) time.Duration {
		return interval
	}
}

func RetryLinear(increase time.Duration) RetryFunc {
	return func(interval time.Duration) time.Duration {
		return interval + increase
	}
}

func RetryGeometric(factor float64) RetryFunc {
	return func(interval time.Duration) time.Duration {
		return time.Duration(float64(interval) * factor)
	}
}

func RetryExponential(exponent float64) RetryFunc {
	return func(interval time.Duration) time.Duration {
		return time.Duration(math.Pow(float64(interval), exponent))
	}
}

type OptionFunc func(*Config)

// WithBatchInterval sets a custom interval at which we scan for pending items
// and batch them for requesting.
func WithBatchInterval(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.BatchInterval = interval
	}
}

// WithBatchThreshold sets a custom threshold for the maximum size of a batch.
// If we have the given amount of pending items, we immediately send a batch.
func WithBatchThreshold(threshold uint) OptionFunc {
	return func(cfg *Config) {
		cfg.BatchThreshold = threshold
	}
}

// WithRetryInitial sets the initial interval for dispatching a request for the
// second time.
func WithRetryInitial(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.RetryInitial = interval
	}
}

// WithRetryFunction sets the function at which the retry interval increases.
func WithRetryFunction(retry RetryFunc) OptionFunc {
	return func(cfg *Config) {
		cfg.RetryFunction = retry
	}
}

// WithRetryMaximum sets the maximum retry interval at which we will retry.
func WithRetryMaximum(interval time.Duration) OptionFunc {
	return func(cfg *Config) {
		cfg.RetryMaximum = interval
	}
}

// WithRetryAttempts sets the number of attempts we will make before we give
// up on retrying. Use zero for infinite retries.
func WithRetryAttempts(attempts uint) OptionFunc {
	return func(cfg *Config) {
		cfg.RetryAttempts = attempts
	}
}

// WithValidateStaking sets the flag which determines if the target and origin must be checked for staking
func WithValidateStaking(validateStaking bool) OptionFunc {
	return func(cfg *Config) {
		cfg.ValidateStaking = validateStaking
	}
}
