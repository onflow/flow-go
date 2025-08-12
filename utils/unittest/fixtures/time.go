package fixtures

import (
	"testing"
	"time"
)

// TimeGenerator generates time.Time values with consistent randomness.
type TimeGenerator struct {
	randomGen *RandomGenerator
}

// timeConfig holds the configuration for time generation.
type timeConfig struct {
	baseTime time.Time
	offset   time.Duration
	timezone *time.Location
}

// WithBaseTime returns an option to set the base time for generation.
func (g *TimeGenerator) WithBaseTime(baseTime time.Time) func(*timeConfig) {
	return func(config *timeConfig) {
		config.baseTime = baseTime
	}
}

// WithTimezone returns an option to set the timezone for time generation.
func (g *TimeGenerator) WithTimezone(tz *time.Location) func(*timeConfig) {
	return func(config *timeConfig) {
		config.timezone = tz
	}
}

// WithOffset returns an option to set the offset from the base time.
func (g *TimeGenerator) WithOffset(offset time.Duration) func(*timeConfig) {
	return func(config *timeConfig) {
		config.offset = offset
	}
}

// WithOffsetRandom returns an option to set a random offset from the base time in the range [0, max).
func (g *TimeGenerator) WithOffsetRandom(max time.Duration) func(*timeConfig) {
	return func(config *timeConfig) {
		offset := time.Duration(g.randomGen.Intn(int(max)))
		config.offset = offset
	}
}

// Fixture generates a time.Time value with optional configuration.
func (g *TimeGenerator) Fixture(t testing.TB, opts ...func(*timeConfig)) time.Time {
	config := &timeConfig{
		baseTime: time.Date(2020, 7, 14, 16, 0, 0, 0, time.UTC), // 2020-07-14T16:00:00Z
		timezone: time.UTC,
	}

	for _, opt := range opts {
		opt(config)
	}

	// Apply timezone if specified
	if config.timezone != config.baseTime.Location() {
		config.baseTime = config.baseTime.In(config.timezone)
	}

	return config.baseTime.Add(config.offset)
}
