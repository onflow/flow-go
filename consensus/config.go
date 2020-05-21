package consensus

import (
	"time"
)

type ParticipantConfig struct {
	TimeoutInitial             time.Duration // the initial timeout for the pacemaker
	TimeoutMinimum             time.Duration // the minimum timeout for the pacemaker
	TimeoutAggregationFraction float64       // the percentage part of the timeout period reserved for vote aggregation
	TimeoutIncreaseFactor      float64       // the factor at which the timeout grows when timeouts occur
	TimeoutDecreaseStep        time.Duration // the step with which the timeout decreases when no timeouts occur
	BlockRateDelay             time.Duration // a delay to broadcast block proposal in order to control the block production rate
}

type Option func(*ParticipantConfig)

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutInitial = timeout
		cfg.TimeoutMinimum = timeout
		cfg.TimeoutDecreaseStep = timeout / 2
	}
}

func WithBlockRateDelay(delay time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.BlockRateDelay = delay
	}
}

func WithTimeoutDecreaseStep(decrease time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutDecreaseStep = decrease
	}
}

func WithVoteAggregationTimeoutFraction(fraction float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutAggregationFraction = fraction
	}
}

func WithTimeoutIncreaseFactor(factor float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutIncreaseFactor = factor
	}
}
