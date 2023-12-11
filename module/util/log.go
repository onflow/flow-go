package util

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// LogProgressFunc is a function that can be called to add to the progress
type LogProgressFunc func(addProgress int)

type LogProgressConfig struct {
	Message string
	Total   int
	Sampler zerolog.Sampler
}

func DefaultLogProgressConfig(
	message string,
	total int,
) LogProgressConfig {
	nth := uint32(total / 10) // sample every 10% by default
	if nth == 0 {
		nth = 1
	}

	sampler := newProgressLogsSampler(nth, 60*time.Second)
	return NewLogProgressConfig(
		message,
		total,
		sampler,
	)
}

func NewLogProgressConfig(
	message string,
	total int,
	sampler zerolog.Sampler) LogProgressConfig {
	return LogProgressConfig{
		Message: message,
		Total:   total,
		Sampler: sampler,
	}

}

type LogProgressOption func(config *LogProgressConfig)

// LogProgress takes a total and return function such that when called adds the given
// number to the progress and logs the progress every 10% or every 60 seconds whichever
// comes first.
// The returned function can be called concurrently.
// An eta is also logged, but it assumes that the progress is linear.
func LogProgress(
	log zerolog.Logger,
	config LogProgressConfig,
) LogProgressFunc {
	sampler := log.Sample(config.Sampler)

	start := time.Now()
	currentIndex := uint64(0)
	return func(add int) {
		current := atomic.AddUint64(&currentIndex, uint64(add))

		percentage := float64(100)
		if config.Total > 0 {
			percentage = (float64(current) / float64(config.Total)) * 100.
		}
		elapsed := time.Since(start)
		elapsedString := elapsed.Round(1 * time.Second).String()

		etaString := "unknown"
		if percentage > 0 {
			eta := time.Duration(float64(elapsed) / percentage * (100 - percentage))
			if eta < 0 {
				eta = 0
			}
			etaString = eta.Round(1 * time.Second).String()

		}

		if current != uint64(config.Total) {
			sampler.Info().Msgf("%s progress %d/%d (%.1f%%) elapsed: %s, eta %s", config.Message, current, config.Total, percentage, elapsedString, etaString)
		} else {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) total time %s", config.Message, current, config.Total, percentage, elapsedString)
		}
	}
}

type TimedSampler struct {
	start    time.Time
	Duration time.Duration
	mu       sync.Mutex
}

var _ zerolog.Sampler = (*TimedSampler)(nil)

func NewTimedSampler(duration time.Duration) *TimedSampler {
	return &TimedSampler{
		start:    time.Now(),
		Duration: duration,
		mu:       sync.Mutex{},
	}
}

func (s *TimedSampler) Sample(_ zerolog.Level) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if time.Since(s.start) > s.Duration {
		s.start = time.Now()
		return true
	}
	return false
}

func (s *TimedSampler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.start = time.Now()
}

type progressLogsSampler struct {
	basicSampler *zerolog.BasicSampler
	timedSampler *TimedSampler
}

var _ zerolog.Sampler = (*progressLogsSampler)(nil)

// newProgressLogsSampler returns a sampler that samples every nth log
// and also samples a log if the last log was more than duration ago
func newProgressLogsSampler(nth uint32, duration time.Duration) zerolog.Sampler {
	return &progressLogsSampler{
		basicSampler: &zerolog.BasicSampler{N: nth},
		timedSampler: NewTimedSampler(duration),
	}
}

func (s *progressLogsSampler) Sample(lvl zerolog.Level) bool {
	sample := s.basicSampler.Sample(lvl)
	if sample {
		s.timedSampler.Reset()
		return true
	}
	return s.timedSampler.Sample(lvl)
}
