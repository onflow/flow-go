package fixtures

import (
	"time"
)

const (
	defaultOffset = int64(10 * 365 * 24 * time.Hour) // 10 years
)

// Time is the default options factory for [time.Time] generation.
var Time timeFactory

type timeFactory struct{}

type TimeOption func(*TimeGenerator, *timeConfig)

// timeConfig holds the configuration for time generation.
type timeConfig struct {
	baseTime time.Time
	offset   time.Duration
	timezone *time.Location
}

// WithBaseTime is an option that sets the time produced by the generator.
// This will be the exact time. If you want to include a random jitter, use the
// WithOffsetRandom option in combination with this option.
func (f timeFactory) WithBaseTime(baseTime time.Time) TimeOption {
	return func(g *TimeGenerator, config *timeConfig) {
		config.baseTime = baseTime
		config.offset = 0
	}
}

// WithTimezone is an option that sets the timezone for time generation.
func (f timeFactory) WithTimezone(tz *time.Location) TimeOption {
	return func(g *TimeGenerator, config *timeConfig) {
		config.timezone = tz
	}
}

// WithOffset is an option that sets the offset from the base time.
// This sets the exact offset of the base time.
func (f timeFactory) WithOffset(offset time.Duration) TimeOption {
	return func(g *TimeGenerator, config *timeConfig) {
		config.offset = offset
	}
}

// WithOffsetRandom is an option that sets a random offset from the base time in the range [0, max).
func (f timeFactory) WithOffsetRandom(max time.Duration) TimeOption {
	return func(g *TimeGenerator, config *timeConfig) {
		config.offset = time.Duration(g.random.Intn(int(max)))
	}
}

// TimeGenerator generates [time.Time] values with consistent randomness.
type TimeGenerator struct {
	timeFactory

	random *RandomGenerator
}

func NewTimeGenerator(
	random *RandomGenerator,
) *TimeGenerator {
	return &TimeGenerator{
		random: random,
	}
}

// Fixture generates a [time.Time] value.
// Uses default base time of 2020-07-14T16:00:00Z and timezone of UTC.
// The default offset is within 10 years of 2020-07-14T16:00:00Z.
func (g *TimeGenerator) Fixture(opts ...TimeOption) time.Time {
	defaultBaseTime := time.Date(2020, 7, 14, 16, 0, 0, 0, time.UTC) // 2020-07-14T16:00:00Z
	defaultOffset := time.Duration(g.random.Int63n(defaultOffset))
	config := &timeConfig{
		baseTime: defaultBaseTime,
		offset:   defaultOffset,
		timezone: time.UTC,
	}

	for _, opt := range opts {
		opt(g, config)
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
func (g *TimeGenerator) List(n int, opts ...TimeOption) []time.Time {
	times := make([]time.Time, n)
	for i := range n {
		times[i] = g.Fixture(opts...)
	}
	return times
}
