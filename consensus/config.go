package consensus

import (
	"time"
)

type ParticipantConfig struct {
	ViewStart                  uint64
	ViewPruned                 uint64
	ViewVoted                  uint64
	TimeoutInitial             time.Duration // the initial timeout for the pacemaker
	TimeoutMinimum             time.Duration // the minimum timeout for the pacemaker
	TimeoutAggregationFraction float64       // the percentage part of the timeout period reserved for vote aggregation
	TimeoutIncreaseFactor      float64       // the factor at which the timeout grows when timeouts occur
	TimeoutDecreaseStep        time.Duration // the step with which the timeout decreases when no timeouts occur
}

type Option func(*ParticipantConfig)

func WithFirstView(view uint64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.ViewStart = view
	}
}

func WithLastPruned(view uint64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.ViewPruned = view
	}
}

func WithLastVoted(view uint64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.ViewVoted = view
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutInitial = timeout
		cfg.TimeoutMinimum = timeout
		cfg.TimeoutDecreaseStep = timeout / 2
	}
}
