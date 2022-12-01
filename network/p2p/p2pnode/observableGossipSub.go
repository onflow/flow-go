package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

// GossipSubControlMessageMetrics is a metrics and observability wrapper component for the incoming RPCs to a
// GossipSub router. It records metrics on the number of control messages received in each RPC.
type GossipSubControlMessageMetrics struct {
	metrics module.GossipSubRouterMetrics
	logger  zerolog.Logger
}

func NewGossipSubControlMessageMetrics(metrics module.GossipSubRouterMetrics, logger zerolog.Logger) *GossipSubControlMessageMetrics {
	return &GossipSubControlMessageMetrics{
		logger:  logger.With().Str("module", "gossipsub-control-message-metrics").Logger(),
		metrics: metrics,
	}
}

// ObserveRPC is invoked to record metrics on incoming RPC messages.
func (o *GossipSubControlMessageMetrics) ObserveRPC(from peer.ID, rpc *pubsub.RPC) {
	lg := o.logger.With().Str("peer_id", from.String()).Logger()

	ctl := rpc.GetControl()
	if ctl == nil {
		lg.Warn().Msg("received rpc with no control message")
		return
	}

	iHaveCount := len(ctl.GetIhave())
	iWantCount := len(ctl.GetIwant())
	graftCount := len(ctl.GetGraft())
	pruneCount := len(ctl.GetPrune())
	includedMessages := len(rpc.GetPublish())

	lg.Trace().
		Int("iHaveCount", iHaveCount).
		Int("iWantCount", iWantCount).
		Int("graftCount", graftCount).
		Int("pruneCount", pruneCount).
		Int("included_message_count", includedMessages).
		Msg("received rpc with control messages")

	o.metrics.OnIHaveReceived(iHaveCount)
	o.metrics.OnIWantReceived(iWantCount)
	o.metrics.OnGraftReceived(graftCount)
	o.metrics.OnPruneReceived(pruneCount)
	o.metrics.OnPublishedGossipMessagesReceived(includedMessages)
}
