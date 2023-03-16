package scoring

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	DefaultAppSpecificScoreWeight = 1
	MaxAppSpecificPenalty         = -100
	MinAppSpecificPenalty         = -1
	MaxAppSpecificReward          = 100

	// DefaultGossipThreshold when a peer's score drops below this threshold,
	// no gossip is emitted towards that peer and gossip from that peer is ignored.
	//
	// Validation Constraint: GossipThreshold >= PublishThreshold && GossipThreshold < 0
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all gossips
	// to and from peers with score -100 are ignored.
	DefaultGossipThreshold = -99

	// DefaultPublishThreshold when a peer's score drops below this threshold,
	// self-published messages are not propagated towards this peer.
	//
	// Validation Constraint:
	// PublishThreshold >= GraylistThreshold && PublishThreshold <= GossipThreshold && PublishThreshold < 0.
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are deprived of
	// receiving any published messages.
	DefaultPublishThreshold = -99

	// DefaultGraylistThreshold when a peer's score drops below this threshold, the peer is graylisted, i.e.,
	// incoming RPCs from the peer are ignored.
	//
	// Validation Constraint:
	// GraylistThreshold =< PublishThreshold && GraylistThreshold =< GossipThreshold && GraylistThreshold < 0
	//
	// How we use it:
	// As current max penalty is -100, we set the threshold to -99 so that all penalized peers are graylisted.
	DefaultGraylistThreshold = -99

	// DefaultAcceptPXThreshold when a peer sends us PX information with a prune, we only accept it and connect to the supplied
	// peers if the originating peer's score exceeds this threshold.
	//
	// Validation Constraint: must be non-negative.
	//
	// How we use it:
	// As current max reward is 100, we set the threshold to 99 so that we only receive supplied peers from
	// well-behaved peers.
	DefaultAcceptPXThreshold = 99

	// DefaultOpportunisticGraftThreshold when the median peer score in the mesh drops below this value,
	// the peer may select more peers with score above the median to opportunistically graft on the mesh.
	//
	// Validation Constraint: must be non-negative.
	//
	// How we use it:
	// We set it to the MaxAppSpecificReward + 1 so that we only opportunistically graft peers that are not access nodes (i.e., with MinAppSpecificPenalty),
	// or penalized peers (i.e., with MaxAppSpecificPenalty).
	DefaultOpportunisticGraftThreshold = MaxAppSpecificReward + 1

	// MaxDebugLogs sets the max number of debug/trace log events per second. Logs emitted above
	// this threshold are dropped.
	MaxDebugLogs = 50
)

// ScoreOption is a functional option for configuring the peer scoring system.
type ScoreOption struct {
	logger                   zerolog.Logger
	validator                *SubscriptionValidator
	idProvider               module.IdentityProvider
	peerScoreParams          *pubsub.PeerScoreParams
	peerThresholdParams      *pubsub.PeerScoreThresholds
	appSpecificScoreFunction func(peer.ID) float64
}

type PeerScoreParamsOption func(option *ScoreOption)

func WithAppSpecificScoreFunction(appSpecificScoreFunction func(peer.ID) float64) PeerScoreParamsOption {
	return func(s *ScoreOption) {
		s.appSpecificScoreFunction = appSpecificScoreFunction
	}
}

func NewScoreOption(logger zerolog.Logger, idProvider module.IdentityProvider, opts ...PeerScoreParamsOption) *ScoreOption {
	throttledSampler := logging.BurstSampler(MaxDebugLogs, time.Second)
	logger = logger.With().
		Str("module", "pubsub_score_option").
		Logger().
		Sample(zerolog.LevelSampler{
			TraceSampler: throttledSampler,
			DebugSampler: throttledSampler,
		})
	validator := NewSubscriptionValidator()
	s := &ScoreOption{
		logger:                   logger,
		validator:                validator,
		idProvider:               idProvider,
		appSpecificScoreFunction: defaultAppSpecificScoreFunction(logger, idProvider, validator),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *ScoreOption) SetSubscriptionProvider(provider *SubscriptionProvider) {
	s.validator.RegisterSubscriptionProvider(provider)
}

func (s *ScoreOption) BuildFlowPubSubScoreOption() pubsub.Option {
	s.preparePeerScoreParams()
	s.preparePeerScoreThresholds()

	s.logger.Info().
		Float64("gossip_threshold", s.peerThresholdParams.GossipThreshold).
		Float64("publish_threshold", s.peerThresholdParams.PublishThreshold).
		Float64("graylist_threshold", s.peerThresholdParams.GraylistThreshold).
		Float64("accept_px_threshold", s.peerThresholdParams.AcceptPXThreshold).
		Float64("opportunistic_graft_threshold", s.peerThresholdParams.OpportunisticGraftThreshold).
		Msg("peer score thresholds configured")

	return pubsub.WithPeerScore(
		s.peerScoreParams,
		s.peerThresholdParams,
	)
}

func (s *ScoreOption) preparePeerScoreThresholds() {
	s.peerThresholdParams = &pubsub.PeerScoreThresholds{
		GossipThreshold:             DefaultGossipThreshold,
		PublishThreshold:            DefaultPublishThreshold,
		GraylistThreshold:           DefaultGraylistThreshold,
		AcceptPXThreshold:           DefaultAcceptPXThreshold,
		OpportunisticGraftThreshold: DefaultOpportunisticGraftThreshold,
	}
}

// preparePeerScoreParams prepares the peer score parameters for the pubsub system.
// It is based on the default parameters defined in libp2p pubsub peer scoring.
func (s *ScoreOption) preparePeerScoreParams() {
	s.peerScoreParams = &pubsub.PeerScoreParams{
		// we don't set all the parameters, so we skip the atomic validation.
		// atomic validation fails initialization if any parameter is not set.
		SkipAtomicValidation: true,

		// DecayInterval is the interval over which we decay the effect of past behavior. So that
		// a good or bad behavior will not have a permanent effect on the score.
		DecayInterval: time.Hour,

		// DecayToZero defines the maximum value below which a peer scoring counter is reset to zero.
		// This is to prevent the counter from decaying to a very small value.
		// The default value is 0.01, which means that a counter will be reset to zero if it decays to 0.01.
		// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
		// for a long time, and we can reset the counter.
		DecayToZero: 0.01,

		// AppSpecificScore is a function that takes a peer ID and returns an application specific score.
		// At the current stage, we only use it to penalize and reward the peers based on their subscriptions.
		AppSpecificScore: s.appSpecificScoreFunction,
		// AppSpecificWeight is the weight of the application specific score.
		AppSpecificWeight: DefaultAppSpecificScoreWeight,
	}
}

func (s *ScoreOption) BuildGossipSubScoreOption() pubsub.Option {
	s.preparePeerScoreParams()
	s.preparePeerScoreThresholds()

	s.logger.Info().
		Float64("gossip_threshold", s.peerThresholdParams.GossipThreshold).
		Float64("publish_threshold", s.peerThresholdParams.PublishThreshold).
		Float64("graylist_threshold", s.peerThresholdParams.GraylistThreshold).
		Float64("accept_px_threshold", s.peerThresholdParams.AcceptPXThreshold).
		Float64("opportunistic_graft_threshold", s.peerThresholdParams.OpportunisticGraftThreshold).
		Msg("peer score thresholds configured")

	return pubsub.WithPeerScore(
		s.peerScoreParams,
		s.peerThresholdParams,
	)
}

func defaultAppSpecificScoreFunction(logger zerolog.Logger, idProvider module.IdentityProvider, validator *SubscriptionValidator) func(peer.ID) float64 {
	return func(pid peer.ID) float64 {
		lg := logger.With().Str("peer_id", pid.String()).Logger()

		// checks if peer has a valid Flow protocol identity.
		flowId, err := HasValidFlowIdentity(idProvider, pid)
		if err != nil {
			lg.Error().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("invalid peer identity, penalizing peer")
			return MaxAppSpecificPenalty
		}

		lg = lg.With().
			Hex("flow_id", logging.ID(flowId.NodeID)).
			Str("role", flowId.Role.String()).
			Logger()

		// checks if peer has any subscription violation.
		if err := validator.CheckSubscribedToAllowedTopics(pid, flowId.Role); err != nil {
			lg.Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("invalid subscription detected, penalizing peer")
			return MaxAppSpecificPenalty
		}

		// checks if peer is an access node, and if so, pushes it to the
		// edges of the network by giving the minimum penalty.
		if flowId.Role == flow.RoleAccess {
			lg.Trace().
				Msg("pushing access node to edge by penalizing with minimum penalty value")
			return MinAppSpecificPenalty
		}

		lg.Trace().
			Msg("rewarding well-behaved non-access node peer with maximum reward value")
		return MaxAppSpecificReward
	}
}
