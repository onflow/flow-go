package util

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// LogProgressFunc is a function that can be called to add to the progress.
// The function can be called concurrently. addProgress is the amount to add to the progress.
// It is an int only for convenience purposes, negative values are ignored.
type LogProgressFunc func(addProgress int)

type LogProgressConfig struct {
	// Message is part of the messages that will be logged.
	// The full template is: `%s progress %d/%d (%.1f%%) total time %s`.
	Message string
	// Total is the total value of progress expected.
	// When Total is added to LogProgressFunc the progress is considered to be 100%.
	Total int
	// NoDataLogDuration. If the last log line was more than this duration ago and a new data point is added, a new log line is logged.
	// No line is logged if no data is received. The minimum resolution for NoDataLogDuration is 1 millisecond.
	NoDataLogDuration time.Duration
	// Ticks is the number of increments to log at. If Total is > 0 there will be at least 2 ticks. One at 0 and one at Total.
	// If you want to log at every 10% set Ticks to 11 (one is at 0%).
	// If the number of ticks is more than Total, it will be set to Total + 1.
	Ticks int
}

// DefaultLogProgressConfig returns a LogProgressConfig with default values.
// The default values will log every 10% and will log an additional line if new data is received
// after no data has been received for 1 minute.
func DefaultLogProgressConfig(
	message string,
	total int,
) LogProgressConfig {
	return LogProgressConfig{
		Message:           message,
		Total:             total,
		Ticks:             11,
		NoDataLogDuration: 60 * time.Second,
	}
}

type LogProgressOption func(config *LogProgressConfig)

// LogProgress takes a LogProgressConfig and return function such that when called adds the given
// number to the progress and logs the progress in defined increments or there is a time gap between progress
// updates.
// The returned function can be called concurrently.
// An eta is also logged, but it assumes that the progress is linear.
func LogProgress(
	log zerolog.Logger,
	config LogProgressConfig,
) LogProgressFunc {

	start := time.Now().UnixMilli()
	var lastDataTime atomic.Int64
	lastDataTime.Store(start)
	var currentIndex atomic.Uint64

	// mutex to protect logProgress from concurrent calls
	// mutex is technically only needed for when the underlying io.Writer for the provider zerolog.Logger
	// is not thread safe. However we lock conservatively because we intend to call logProgress infrequently in normal
	// usage anyway.
	var mux sync.Mutex

	logProgress := func(current uint64) {
		mux.Lock()
		defer mux.Unlock()

		elapsed := time.Since(time.UnixMilli(start))
		elapsedString := elapsed.Round(1 * time.Second).String()

		percentage := float64(100)
		if config.Total > 0 {
			percentage = (float64(current) / float64(config.Total)) * 100.
		}

		etaString := "unknown"
		if percentage > 0 {
			eta := time.Duration(float64(elapsed) / percentage * (100 - percentage))
			if eta < 0 {
				eta = 0
			}
			etaString = eta.Round(1 * time.Second).String()
		}

		if current < uint64(config.Total) {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) elapsed: %s, eta %s", config.Message, current, config.Total, percentage, elapsedString, etaString)
		} else {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) total time %s", config.Message, current, config.Total, percentage, elapsedString)
		}
	}

	// log 0% progress
	logProgress(0)

	// sanitize inputs and calculate increment
	total := uint64(config.Total)
	ticksIncludingZero := uint64(config.Ticks)
	if ticksIncludingZero < 2 {
		ticksIncludingZero = 2
	}
	ticks := ticksIncludingZero - 1

	increment := total / ticks
	if increment == 0 {
		increment = 1
	}

	// increment doesnt necessarily divide config.Total
	// Because we want 100% to mean 100% we need to deduct this overflow from the current value
	// before checking if it is a multiple of the increment.
	incrementsOverflow := total % increment
	noLogDurationMillis := config.NoDataLogDuration.Milliseconds()

	return func(add int) {
		if add < 0 {
			return
		}
		diff := uint64(add)
		now := time.Now().UnixMilli()

		current := currentIndex.Add(diff)
		lastTime := lastDataTime.Swap(now)

		// if the diff went over one or more increments, log the progress for each increment
		fromTick := (current - diff - incrementsOverflow) / increment
		toTick := (current - incrementsOverflow) / increment

		if fromTick == toTick && now-lastTime > noLogDurationMillis {
			// no data for a while, log whatever we are at now
			logProgress(current)
			return
		}

		for t := fromTick; t < toTick; t++ {
			// (t+1) because we want to log the progress for the increment reached
			// not the increment past
			current := increment*(t+1) + incrementsOverflow
			logProgress(current)
		}
	}
}
