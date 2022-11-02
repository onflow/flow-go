package p2pnode

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/module"
)

type ObservableGossipSubRouter struct {
	router *pubsub.GossipSubRouter
}

func NewObservableGossipSub(h host.Host, metrics module.GossipSubRouterMetrics) *ObservableGossipSubRouter {
	return &ObservableGossipSubRouter{
		router: pubsub.DefaultGossipSubRouter(h),
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
