package scoring

import (
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/scoring/internal"
	"github.com/onflow/flow-go/utils/logging"
)

// SubscriptionProvider provides a list of topics a peer is subscribed to.
type SubscriptionProvider struct {
	component.Component
	logger              zerolog.Logger
	topicProviderOracle func() p2p.TopicProvider

	// TODO: we should add an expiry time to this cache and clean up the cache periodically
	// to avoid leakage of stale topics.
	cache SubscriptionCache

	// idProvider translates the peer ids to flow ids.
	idProvider module.IdentityProvider

	allTopicsUpdateInterval time.Duration // the interval for updating the list of topics in the pubsub network that this node has subscribed to.
}

type SubscriptionProviderConfig struct {
	Logger                  zerolog.Logger                            `validate:"required"`
	TopicProviderOracle     func() p2p.TopicProvider                  `validate:"required"`
	IdProvider              module.IdentityProvider                   `validate:"required"`
	HeroCacheMetricsFactory metrics.HeroCacheMetricsFactory           `validate:"required"`
	Params                  *p2pconfig.SubscriptionProviderParameters `validate:"required"`
	NetworkingType          network.NetworkingType                    `validate:"required"`
}

var _ p2p.SubscriptionProvider = (*SubscriptionProvider)(nil)

func NewSubscriptionProvider(cfg *SubscriptionProviderConfig) (*SubscriptionProvider, error) {
	if err := validator.New().Struct(cfg); err != nil {
		return nil, fmt.Errorf("invalid subscription provider config: %w", err)
	}

	cacheMetrics := metrics.NewSubscriptionRecordCacheMetricsFactory(cfg.HeroCacheMetricsFactory, cfg.NetworkingType)
	cache := internal.NewSubscriptionRecordCache(cfg.Params.CacheSize, cfg.Logger, cacheMetrics)

	p := &SubscriptionProvider{
		logger:                  cfg.Logger.With().Str("module", "subscription_provider").Logger(),
		topicProviderOracle:     cfg.TopicProviderOracle,
		allTopicsUpdateInterval: cfg.Params.UpdateInterval,
		idProvider:              cfg.IdProvider,
		cache:                   cache,
	}

	builder := component.NewComponentManagerBuilder()
	p.Component = builder.AddWorker(
		func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			p.logger.Debug().
				Float64("update_interval_seconds", cfg.Params.UpdateInterval.Seconds()).
				Msg("subscription provider started; starting update topics loop")
			p.updateTopicsLoop(ctx)

			<-ctx.Done()
			p.logger.Debug().Msg("subscription provider stopped; stopping update topics loop")
		}).Build()

	return p, nil
}

func (s *SubscriptionProvider) updateTopicsLoop(ctx irrecoverable.SignalerContext) {
	ticker := time.NewTicker(s.allTopicsUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.updateTopics(); err != nil {
				ctx.Throw(fmt.Errorf("update loop failed: %w", err))
				return
			}
		}
	}
}

// updateTopics queries the pubsub network for all topics this node subscribes to and updates the cache
// with the current peer subscriptions for each topic.
//
// IMPORTANT: This method is designed to be called only from the single-goroutine ticker loop in updateTopicsLoop.
// No synchronization is needed because the ticker guarantees sequential execution.
//
// No errors are expected during normal operation.
func (s *SubscriptionProvider) updateTopics() error {
	allTopics := s.topicProviderOracle().GetTopics()
	s.logger.Trace().Msgf("all topics updated: %v", allTopics)

	// increments the update cycle of the cache; so that the previous cache entries are invalidated upon a read or write.
	s.cache.MoveToNextUpdateCycle()
	for _, topic := range allTopics {
		peers := s.topicProviderOracle().ListPeers(topic)

		for _, p := range peers {
			if _, authorized := s.idProvider.ByPeerID(p); !authorized {
				// peer is not authorized (staked); hence it does not have a valid role in the network; and
				// we skip the topic update for this peer (also avoiding sybil attacks on the cache).
				s.logger.Debug().
					Str("remote_peer_id", p2plogging.PeerId(p)).
					Bool(logging.KeyNetworkingSecurity, true).
					Msg("skipping topic update for unauthorized peer")
				continue
			}

			updatedTopics, err := s.cache.AddWithInitTopicForPeer(p, topic)
			if err != nil {
				// this is an irrecoverable error; hence, we crash the node.
				return fmt.Errorf("failed to update topics for peer %s: %w", p, err)
			}
			s.logger.Debug().
				Str("remote_peer_id", p2plogging.PeerId(p)).
				Strs("updated_topics", updatedTopics).
				Msg("updated topics for peer")
		}
	}
	return nil
}

// GetSubscribedTopics returns all the subscriptions of a peer within the pubsub network.
func (s *SubscriptionProvider) GetSubscribedTopics(pid peer.ID) []string {
	topics, ok := s.cache.GetSubscribedTopics(pid)
	if !ok {
		s.logger.Trace().Str("peer_id", p2plogging.PeerId(pid)).Msg("no topics found for peer")
		return nil
	}
	return topics
}
