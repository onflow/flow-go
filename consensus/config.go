package consensus

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
)

// HotstuffModules is a helper structure to encapsulate dependencies to create
// a hotStuff participant.
type HotstuffModules struct {
	Notifier                    hotstuff.Consumer               // observer for hotstuff events
	Committee                   hotstuff.DynamicCommittee       // consensus committee
	Signer                      hotstuff.Signer                 // signer of proposal & votes
	Persist                     hotstuff.Persister              // last state of consensus participant
	FinalizationDistributor     *pubsub.FinalizationDistributor // observer for finalization events, used by compliance engine
	QCCreatedDistributor        *pubsub.QCCreatedDistributor    // observer for qc created event, used by leader
	TimeoutCollectorDistributor *pubsub.TimeoutCollectorDistributor
	Forks                       hotstuff.Forks             // information about multiple forks
	Validator                   hotstuff.Validator         // validator of proposals & votes
	VoteAggregator              hotstuff.VoteAggregator    // aggregator of votes, used by leader
	TimeoutAggregator           hotstuff.TimeoutAggregator // aggregator of `TimeoutObject`s, used by every replica
}

type ParticipantConfig struct {
	StartupTime           time.Time     // the time when consensus participant enters first view
	TimeoutMinimum        time.Duration // the minimum timeout for the pacemaker
	TimeoutMaximum        time.Duration // the maximum timeout for the pacemaker
	TimeoutIncreaseFactor float64       // the factor at which the timeout grows when timeouts occur
	HappyPathRounds       uint64        // number of failed rounds before first timeout increase
	BlockRateDelay        time.Duration // a delay to broadcast block proposal in order to control the block production rate
}

type Option func(*ParticipantConfig)

func WithStartupTime(time time.Time) Option {
	return func(cfg *ParticipantConfig) {
		cfg.StartupTime = time
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

func WithHappyPathRounds(happyPathRounds uint64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.HappyPathRounds = happyPathRounds
	}
}

func WithBlockRateDelay(delay time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.BlockRateDelay = delay
	}
}
