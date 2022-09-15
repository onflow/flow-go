package scoring

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
)

const (
	DefaultAppSpecificScoreWeight = 1
	MaxAppSpecificPenalty         = -100
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
	// We set it to 0 so that we can opportunistically graft peers when half of the mesh is penalized.
	DefaultOpportunisticGraftThreshold = 0
)

// ScoreOption is a functional option for configuring the peer scoring system.
type ScoreOption struct {
	logger              zerolog.Logger
	validator           *SubscriptionValidator
	peerScoreParams     *pubsub.PeerScoreParams
	peerThresholdParams *pubsub.PeerScoreThresholds
}

func NewScoreOption(logger zerolog.Logger) *ScoreOption {
	return &ScoreOption{
		logger: logger.With().Str("module", "pubsub_score_option").Logger(),
	}
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

func (s *ScoreOption) preparePeerScoreParams() {
	s.peerScoreParams = &pubsub.PeerScoreParams{
		AppSpecificScore: func(pid peer.ID) float64 {
			if err := s.validator.MustSubscribedToAllowedTopics(pid); err != nil {
				s.logger.Error().
					Err(err).
					Str("peer_id", pid.String()).
					Msg("invalid subscription detected, penalizing peer")
				return MaxAppSpecificPenalty
			}
			s.logger.Trace().
				Str("peer_id", pid.String()).
				Msg("subscribed topics for peer validated, rewarding peer")
			return MaxAppSpecificReward
		},
		AppSpecificWeight: DefaultAppSpecificScoreWeight,
	}
}
