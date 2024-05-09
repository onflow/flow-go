package logging

import (
	"time"

	"github.com/rs/zerolog"
)

type BurstSamplerOption func(*zerolog.BurstSampler)

// WithNextSampler adds a sampler that is applied after exceeding the burst limit
func WithNextSampler(sampler zerolog.Sampler) BurstSamplerOption {
	return func(s *zerolog.BurstSampler) {
		s.NextSampler = sampler
	}
}

// BurstSampler returns a zerolog.BurstSampler with the provided burst and interval.
// Logs emitted beyond the burst limit are dropped
func BurstSampler(burst uint32, interval time.Duration, opts ...BurstSamplerOption) *zerolog.BurstSampler {
	s := &zerolog.BurstSampler{
		Burst:  burst,
		Period: interval,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}
