package util

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// LogProgress takes a total and return function such that when called with a 0-based index
// it prints the progress from 0% to 100% to indicate the index from 0 to (total - 1) has been
// processed.
// useful to report the progress of processing the index from 0 to (total - 1)
func LogProgress(msg string, total int, log zerolog.Logger) func(currentIndex int) {
	nth := uint32(total / 10) // sample every 10% by default
	if nth == 0 {
		nth = 1
	}

	sampler := log.Sample(newProgressLogsSampler(nth, 60*time.Second))

	start := time.Now()
	return func(currentIndex int) {
		percentage := float64(100)
		if total > 0 {
			percentage = (float64(currentIndex+1) / float64(total)) * 100. // currentIndex+1 assuming zero based indexing
		}

		etaString := "unknown"
		if percentage > 0 {
			eta := time.Duration(float64(time.Since(start)) / percentage * (100 - percentage))
			etaString = eta.String()

		}

		if currentIndex+1 != total {
			sampler.Info().Msgf("%s progress %d/%d (%.1f%%) eta %s", msg, currentIndex+1, total, percentage, etaString)
		} else {
			log.Info().Msgf("%s progress %d/%d (%.1f%%) total time %s", msg, currentIndex+1, total, percentage, time.Since(start))
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
