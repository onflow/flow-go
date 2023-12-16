package scoring

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/logging"
	p2putils "github.com/onflow/flow-go/network/p2p/utils"
)

// SubscriptionValidator validates that a peer is subscribed to topics that it is allowed to subscribe to.
// It is used to penalize peers that subscribe to topics that they are not allowed to subscribe to in GossipSub.
type SubscriptionValidator struct {
	component.Component
	logger               zerolog.Logger
	subscriptionProvider p2p.SubscriptionProvider
}

func NewSubscriptionValidator(logger zerolog.Logger, provider p2p.SubscriptionProvider) *SubscriptionValidator {
	v := &SubscriptionValidator{
		logger:               logger.With().Str("component", "subscription_validator").Logger(),
		subscriptionProvider: provider,
	}

	v.Component = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			logger.Debug().Msg("starting subscription validator")
			v.subscriptionProvider.Start(ctx)
			select {
			case <-ctx.Done():
				logger.Debug().Msg("subscription validator is stopping")
			case <-v.subscriptionProvider.Ready():
				logger.Debug().Msg("subscription validator started")
				ready()
				logger.Debug().Msg("subscription validator is ready")
			}

			<-ctx.Done()
			logger.Debug().Msg("subscription validator is stopping")
			<-v.subscriptionProvider.Done()
			logger.Debug().Msg("subscription validator stopped")
		}).Build()

	return v
}

var _ p2p.SubscriptionValidator = (*SubscriptionValidator)(nil)

// CheckSubscribedToAllowedTopics checks if a peer is subscribed to topics that it is allowed to subscribe to.
// Args:
//
//		pid: the peer ID of the peer to check
//	 role: the role of the peer to check
//
// Returns:
// error: if the peer is subscribed to topics that it is not allowed to subscribe to, an InvalidSubscriptionError is returned.
// The error is benign, i.e., it does not indicate an illegal state in the execution of the code. We expect this error
// when there are malicious peers in the network. But such errors should not lead to a crash of the node.
func (v *SubscriptionValidator) CheckSubscribedToAllowedTopics(pid peer.ID, role flow.Role) error {
	lg := v.logger.With().Str("remote_peer_id", p2plogging.PeerId(pid)).Logger()

	topics := v.subscriptionProvider.GetSubscribedTopics(pid)
	lg.Trace().Strs("topics", topics).Msg("checking subscription for remote peer id")

	for _, topic := range topics {
		if !p2putils.AllowedSubscription(role, topic) {
			return p2p.NewInvalidSubscriptionError(topic)
		}
	}

	lg.Trace().Msg("subscription is valid")
	return nil
}
