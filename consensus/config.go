package consensus

import (
	"time"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
)

// HotstuffModules is a helper structure to encapsulate dependencies to create
// a hotStuff participant.
type HotstuffModules struct {
	Committee                   hotstuff.DynamicCommittee           // consensus committee
	Signer                      hotstuff.Signer                     // signer of proposal & votes
	Persist                     hotstuff.Persister                  // last state of consensus participant
	Notifier                    *pubsub.Distributor                 // observer for hotstuff events
	VoteCollectorDistributor    *pubsub.VoteCollectorDistributor    // observer for vote aggregation events, used by leader
	TimeoutCollectorDistributor *pubsub.TimeoutCollectorDistributor // observer for timeout aggregation events
	Forks                       hotstuff.Forks                      // information about multiple forks
	Validator                   hotstuff.Validator                  // validator of proposals & votes
	VoteAggregator              hotstuff.VoteAggregator             // aggregator of votes, used by leader
	TimeoutAggregator           hotstuff.TimeoutAggregator          // aggregator of `TimeoutObject`s, used by every replica
}

type ParticipantConfig struct {
	StartupTime                         time.Time                // the time when consensus participant enters first view
	TimeoutMinimum                      time.Duration            // the minimum timeout for the pacemaker
	TimeoutMaximum                      time.Duration            // the maximum timeout for the pacemaker
	TimeoutAdjustmentFactor             float64                  // the factor at which the timeout duration is adjusted
	HappyPathMaxRoundFailures           uint64                   // number of failed rounds before first timeout increase
	MaxTimeoutObjectRebroadcastInterval time.Duration            // maximum interval for timeout object rebroadcast
	ProposalDurationProvider            ProposalDurationProvider // a delay to broadcast block proposal in order to control the block production rate
}

func DefaultParticipantConfig() ParticipantConfig {
	defTimeout := timeout.DefaultConfig
	cfg := ParticipantConfig{
		TimeoutMinimum:                      time.Duration(defTimeout.MinReplicaTimeout) * time.Millisecond,
		TimeoutMaximum:                      time.Duration(defTimeout.MaxReplicaTimeout) * time.Millisecond,
		TimeoutAdjustmentFactor:             defTimeout.TimeoutAdjustmentFactor,
		HappyPathMaxRoundFailures:           defTimeout.HappyPathMaxRoundFailures,
		MaxTimeoutObjectRebroadcastInterval: time.Duration(defTimeout.MaxTimeoutObjectRebroadcastInterval) * time.Millisecond,
		ProposalDurationProvider:            staticProposalDurationProvider{dur: 0},
	}
	return cfg
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

func WithTimeoutAdjustmentFactor(factor float64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.TimeoutAdjustmentFactor = factor
	}
}

func WithHappyPathMaxRoundFailures(happyPathMaxRoundFailures uint64) Option {
	return func(cfg *ParticipantConfig) {
		cfg.HappyPathMaxRoundFailures = happyPathMaxRoundFailures
	}
}

func WithProposalDurationProvider(provider ProposalDurationProvider) Option {
	return func(cfg *ParticipantConfig) {
		cfg.ProposalDurationProvider = provider
	}
}

func WithStaticProposalDuration(dur time.Duration) Option {
	return func(cfg *ParticipantConfig) {
		cfg.ProposalDurationProvider = staticProposalDurationProvider{dur: dur}
	}
}

// ProposalDurationProvider provides the ProposalDelay to the Pacemaker.
// The ProposalDelay is the time a leader should attempt to consume between
// entering a view and broadcasting its proposal for that view.
type ProposalDurationProvider interface {
	ProposalDuration() time.Duration
}

// staticProposalDurationProvider is a ProposalDurationProvider which provides a static ProposalDuration.
type staticProposalDurationProvider struct {
	dur time.Duration
}

func (p staticProposalDurationProvider) ProposalDuration() time.Duration {
	return p.dur
}
