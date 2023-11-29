package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/p2plogging"
)

// GossipSubControlMessageMetrics is a metrics and observability wrapper component for the incoming RPCs to a
// GossipSub router. It records metrics on the number of control messages received in each RPC.
type GossipSubControlMessageMetrics struct {
	metrics module.GossipSubRpcInspectorMetrics
	logger  zerolog.Logger
}

var _ p2p.GossipSubControlMetricsObserver = (*GossipSubControlMessageMetrics)(nil)

func NewGossipSubControlMessageMetrics(metrics module.GossipSubRpcInspectorMetrics, logger zerolog.Logger) *GossipSubControlMessageMetrics {
	return &GossipSubControlMessageMetrics{
		logger:  logger.With().Str("module", "gossipsub-control-message-metrics").Logger(),
		metrics: metrics,
	}
}

// ObserveRPC is invoked to record metrics on incoming RPC messages.
func (o *GossipSubControlMessageMetrics) ObserveRPC(from peer.ID, rpc *pubsub.RPC) {
	lg := o.logger.With().Str("peer_id", p2plogging.PeerId(from)).Logger()
	includedMessages := len(rpc.GetPublish())

	ctl := rpc.GetControl()
	if ctl == nil && includedMessages == 0 {
		lg.Trace().Msg("received rpc with no control message and no publish messages")
		return
	}

	iHaveCount := len(ctl.GetIhave())
	iWantCount := len(ctl.GetIwant())
	graftCount := len(ctl.GetGraft())
	pruneCount := len(ctl.GetPrune())

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
