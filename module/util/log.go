package util

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// LogProgressFunc is a function that can be called to add to the progress.
// The function can be called concurrently. addProgress is the amount to add to the progress.
// It is any integer number type, but all negative values are ignored.
type LogProgressFunc[T int | uint | int32 | uint32 | uint64 | int64] func(addProgress T)

type LogProgressConfig[T int | uint | int32 | uint32 | uint64 | int64] struct {
	// message is part of the messages that will be logged.
	// The full template is: `%s progress %d/%d (%.1f%%) total time %s`.
	message string
	// total is the total value of progress expected.
	// When total is added to LogProgressFunc the progress is considered to be 100%.
	total T
	// noDataLogDuration. If the last log line was more than this duration ago and a new data point is added, a new log line is logged.
	// No line is logged if no data is received. The minimum resolution for noDataLogDuration is 1 millisecond.
	noDataLogDuration time.Duration
	// ticks is the number of increments to log at. If total is > 0 there will be at least 2 ticks. One at 0 and one at total.
	// If you want to log at every 10% set ticks to 11 (one is at 0%).
	// If the number of ticks is more than total, it will be set to total + 1.
	ticks uint64
}

// DefaultLogProgressConfig returns a LogProgressConfig with default values.
// The default values will log every 10% and will log an additional line if new data is received
// after no data has been received for 1 minute.
func DefaultLogProgressConfig[T int | uint | int32 | uint32 | uint64 | int64](
	message string,
	total T,
) LogProgressConfig[T] {
	return NewLogProgressConfig[T](
		message,
		total,
		60*time.Second,
		10,
	)
}

// NewLogProgressConfig creates and returns a new LogProgressConfig with the specified message, total, duration, and ticks.
// The duration is rounded to the nearest millisecond.
// The number of ticks is the number of increments to log at. Logging at 0% is always done.
// If you want to log at 10% increments, set ticks to 10.
func NewLogProgressConfig[T int | uint | int32 | uint32 | uint64 | int64](
	message string,
	total T,
	noDataLogDuration time.Duration,
	ticks uint64,
) LogProgressConfig[T] {
	// sanitize total
	if total < 0 {
		total = 0
	}

	// add the tick at 0%
	ticks = ticks + 1

	// sanitize ticks
	// number of ticks should be at most total + 1
	if uint64(total+1) < ticks {
		ticks = uint64(total + 1)
	}

	// sanitize noDataLogDuration
	if noDataLogDuration < time.Millisecond {
		noDataLogDuration = time.Millisecond
	}

	return LogProgressConfig[T]{
		message:           message,
		total:             total,
		noDataLogDuration: noDataLogDuration,
		ticks:             ticks,
	}

}

// LogProgress takes a LogProgressConfig and return function such that when called adds the given
// number to the progress and logs the progress in defined increments or there is a time gap between progress
// updates.
// The returned function can be called concurrently.
// An eta is also logged, but it assumes that the progress is linear.
func LogProgress[T int | uint | int32 | uint32 | uint64 | int64](
	log zerolog.Logger,
	config LogProgressConfig[T],
) LogProgressFunc[T] {

	start := time.Now().UnixMilli()
	var lastDataTime atomic.Int64
	lastDataTime.Store(start)
	var currentIndex atomic.Uint64

	// mutex to protect logProgress from concurrent calls
	// mutex is technically only needed for when the underlying io.Writer for the provider zerolog.Logger
	// is not thread safe. However we lock conservatively because we intend to call logProgress infrequently in normal
	// usage anyway.
	var mux sync.Mutex

	total := uint64(config.total)
	logProgress := func(current uint64) {
		mux.Lock()
		defer mux.Unlock()

		elapsed := time.Since(time.UnixMilli(start))
		elapsedString := elapsed.Round(1 * time.Second).String()

		percentage := float64(100)
		if config.total > 0 {
			percentage = (float64(current) / float64(config.total)) * 100.
		}

		etaString := "unknown"
		if percentage > 0 {
			eta := time.Duration(float64(elapsed) / percentage * (100 - percentage))
			if eta < 0 {
				eta = 0
			}
			etaString = eta.Round(1 * time.Second).String()
		}

		if current < total {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) elapsed: %s, eta %s", config.message, current, config.total, percentage, elapsedString, etaString)
		} else {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) total time %s", config.message, current, config.total, percentage, elapsedString)
		}
	}

	// log 0% progress
	logProgress(0)

	// sanitize inputs and calculate increment
	ticksIncludingZero := config.ticks
	if ticksIncludingZero < 2 {
		ticksIncludingZero = 2
	}
	ticks := ticksIncludingZero - 1

	increment := total / ticks
	if increment == 0 {
		increment = 1
	}

	// increment doesn't necessarily divide config.total
	// Because we want 100% to mean 100% we need to deduct this overflow from the current value
	// before checking if it is a multiple of the increment.
	incrementsOverflow := total % increment
	noLogDurationMillis := config.noDataLogDuration.Milliseconds()

	return func(add T) {
		if total == 0 {
			return
		}
		if add < 0 {
			return
		}
		diff := uint64(add)
		now := time.Now().UnixMilli()

		// it can technically happen that current > total. In this case we continue to log
		// the progress using the calculated increments
		current := currentIndex.Add(diff)
		lastTime := lastDataTime.Swap(now)

		// if the diff went over one or more increments, log the progress for each increment
		fromTick := uint64(0)
		if current-diff >= incrementsOverflow {
			fromTick = (current - diff - incrementsOverflow) / increment
		}
		toTick := uint64(0)
		if current >= incrementsOverflow {
			toTick = (current - incrementsOverflow) / increment
		}

		if fromTick == toTick && now-lastTime > noLogDurationMillis {
			// no data for a while, log whatever we are at now
			logProgress(current)
			return
		}

		if toTick <= fromTick {
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
