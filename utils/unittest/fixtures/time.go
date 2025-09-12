package fixtures

import (
	"time"
)

const (
	defaultOffset = int64(10 * 365 * 24 * time.Hour) // 10 years
)

// TimeGenerator generates [time.Time] values with consistent randomness.
type TimeGenerator struct {
	randomGen *RandomGenerator
}

func NewTimeGenerator(
	randomGen *RandomGenerator,
) *TimeGenerator {
	return &TimeGenerator{
		randomGen: randomGen,
	}
}

// timeConfig holds the configuration for time generation.
type timeConfig struct {
	baseTime time.Time
	offset   time.Duration
	timezone *time.Location
}

// WithBaseTime is an option that sets the base time for generation.
func (g *TimeGenerator) WithBaseTime(baseTime time.Time) func(*timeConfig) {
	return func(config *timeConfig) {
		config.baseTime = baseTime
	}
}

// WithTimezone is an option that sets the timezone for time generation.
func (g *TimeGenerator) WithTimezone(tz *time.Location) func(*timeConfig) {
	return func(config *timeConfig) {
		config.timezone = tz
	}
}

// WithOffset is an option that sets the offset from the base time.
func (g *TimeGenerator) WithOffset(offset time.Duration) func(*timeConfig) {
	return func(config *timeConfig) {
		config.offset = offset
	}
}

// WithOffsetRandom is an option that sets a random offset from the base time in the range [0, max).
func (g *TimeGenerator) WithOffsetRandom(max time.Duration) func(*timeConfig) {
	return func(config *timeConfig) {
		offset := time.Duration(g.randomGen.Intn(int(max)))
		config.offset = offset
	}
}

// Fixture generates a [time.Time] value.
// Uses default base time of 2020-07-14T16:00:00Z and timezone of UTC.
// The default offset is within 10 years of 2020-07-14T16:00:00Z.
func (g *TimeGenerator) Fixture(opts ...func(*timeConfig)) time.Time {
	config := &timeConfig{
		baseTime: time.Date(2020, 7, 14, 16, 0, 0, 0, time.UTC), // 2020-07-14T16:00:00Z
		offset:   time.Duration(g.randomGen.Int63n(defaultOffset)),
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

// List generates a list of [time.Time] values.
// Uses default base time of 2020-07-14T16:00:00Z and timezone of UTC.
// The default offset is within 10 years of 2020-07-14T16:00:00Z.
func (g *TimeGenerator) List(n int, opts ...func(*timeConfig)) []time.Time {
	times := make([]time.Time, n)
	for i := range n {
		times[i] = g.Fixture(opts...)
	}
	return times
}
