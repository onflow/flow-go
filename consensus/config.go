package consensus

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

type ParticipantConfig struct {
	TimeoutInitial             time.Duration       // the initial timeout for the pacemaker
	TimeoutMinimum             time.Duration       // the minimum timeout for the pacemaker
	TimeoutAggregationFraction float64             // the percentage part of the timeout period reserved for vote aggregation
	TimeoutIncreaseFactor      float64             // the factor at which the timeout grows when timeouts occur
	TimeoutDecreaseFactor      float64             // the factor at which the timeout grows when timeouts occur
	BlockRateDelay             time.Duration       // a delay to broadcast block proposal in order to control the block production rate
	BlockTimer                 hotstuff.BlockTimer // validator of block timestamps
}

type Option func(*ParticipantConfig)

func WithInitialTimeout(timeout time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutInitial = timeout
	}
}

func WithMinTimeout(timeout time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutMinimum = timeout
	}
}

func WithTimeoutIncreaseFactor(factor float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutIncreaseFactor = factor
	}
}

func WithTimeoutDecreaseFactor(factor float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutDecreaseFactor = factor
	}
}

func WithVoteAggregationTimeoutFraction(fraction float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutAggregationFraction = fraction
	}
}

func WithBlockRateDelay(delay time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.BlockRateDelay = delay
	}
}

func WithBlockTimer(timer hotstuff.BlockTimer) Option {
	return func(cfg *ParticipantConfig) {
		cfg.BlockTimer = timer
	}
}
