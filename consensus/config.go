package consensus

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
)

// HotstuffModules is a helper structure to encapsulate dependencies to create
// a hotStuff participant.
type HotstuffModules struct {
	Notifier                hotstuff.Consumer               // observer for hotstuff events
	Committee               hotstuff.Committee              // consensus committee
	Signer                  hotstuff.Signer                 // signer of proposal & votes
	Persist                 hotstuff.Persister              // last state of consensus participant
	FinalizationDistributor *pubsub.FinalizationDistributor // observer for finalization events, used by compliance engine
	QCCreatedDistributor    *pubsub.QCCreatedDistributor    // observer for qc created event, used by leader
	Forks                   hotstuff.Forks                  // information about multiple forks
	Validator               hotstuff.Validator              // validator of proposals & votes
	Aggregator              hotstuff.VoteAggregator         // aggregator of votes, used by leader
}

type ParticipantConfig struct {
	StartupTime                time.Time     // the time when consensus participant enters first view
	TimeoutInitial             time.Duration // the initial timeout for the pacemaker
	TimeoutMinimum             time.Duration // the minimum timeout for the pacemaker
	TimeoutAggregationFraction float64       // the percentage part of the timeout period reserved for vote aggregation
	TimeoutIncreaseFactor      float64       // the factor at which the timeout grows when timeouts occur
	TimeoutDecreaseFactor      float64       // the factor at which the timeout grows when timeouts occur
	BlockRateDelay             time.Duration // a delay to broadcast block proposal in order to control the block production rate
}

type Option func(*ParticipantConfig)

func WithStartupTime(time time.Time) Option {
	return func(cfg *ParticipantConfig) {
		cfg.StartupTime = time
	}
}

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
