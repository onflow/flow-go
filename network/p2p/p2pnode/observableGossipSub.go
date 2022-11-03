package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
)

type ObservableGossipSubRouter struct {
	router  *pubsub.GossipSubRouter
	metrics module.GossipSubRouterMetrics
	logger  zerolog.Logger
}

func NewObservableGossipSub(h host.Host, metrics module.GossipSubRouterMetrics, logger zerolog.Logger) *ObservableGossipSubRouter {
	return &ObservableGossipSubRouter{
		router:  pubsub.DefaultGossipSubRouter(h),
		logger:  logger.With().Str("module", "observable-gossipsub-router").Logger(),
		metrics: metrics,
	}
}

var _ pubsub.PubSubRouter = (*ObservableGossipSubRouter)(nil)

func (o *ObservableGossipSubRouter) Protocols() []protocol.ID {
	return o.router.Protocols()
}

func (o *ObservableGossipSubRouter) Attach(sub *pubsub.PubSub) {
	o.router.Attach(sub)
}

func (o *ObservableGossipSubRouter) AddPeer(id peer.ID, protocol protocol.ID) {
	o.router.AddPeer(id, protocol)
}

func (o *ObservableGossipSubRouter) RemovePeer(id peer.ID) {
	o.router.RemovePeer(id)
}

func (o *ObservableGossipSubRouter) EnoughPeers(topic string, suggested int) bool {
	return o.router.EnoughPeers(topic, suggested)
}

func (o *ObservableGossipSubRouter) AcceptFrom(id peer.ID) pubsub.AcceptStatus {
	return o.router.AcceptFrom(id)
}

func (o *ObservableGossipSubRouter) HandleRPC(rpc *pubsub.RPC) {
	ctl := rpc.GetControl()
	if ctl == nil {
		o.logger.Warn().Msg("received rpc with no control message")
		return
	}

	iHaveCount := len(ctl.GetIhave())
	iWantCount := len(ctl.GetIwant())
	graftCount := len(ctl.GetGraft())
	pruneCount := len(ctl.GetPrune())

	// TODO: add peer id of the sender to the log (currently unavailable in the RPC).
	o.logger.Debug().
		Int("iHaveCount", iHaveCount).
		Int("iWantCount", iWantCount).
		Int("graftCount", graftCount).
		Int("pruneCount", pruneCount).
		Msg("received rpc with control messages")

	o.metrics.OnRpcReceived(0)
	o.metrics.OnIHaveReceived(iHaveCount)
	o.metrics.OnIWantReceived(iWantCount)
	o.metrics.OnGraftReceived(graftCount)
	o.metrics.OnPruneReceived(pruneCount)

	o.router.HandleRPC(rpc)
}

func (o *ObservableGossipSubRouter) Publish(message *pubsub.Message) {
	o.router.Publish(message)
}

func (o *ObservableGossipSubRouter) Join(topic string) {
	o.router.Join(topic)
}

func (o *ObservableGossipSubRouter) Leave(topic string) {
	o.router.Leave(topic)
}

func (o *ObservableGossipSubRouter) WithDefaultTagTracer() pubsub.Option {
	return o.router.WithDefaultTagTracer()
}
