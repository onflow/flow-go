package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

type CorruptGossipSubRouter struct {
	router *pubsub.GossipSubRouter
}

var _ pubsub.GossipPubSubRouter = (*CorruptGossipSubRouter)(nil)

func NewCorruptGossipSubRouter(router *pubsub.GossipSubRouter) *CorruptGossipSubRouter {
	return &CorruptGossipSubRouter{
		router: router,
	}
}

func (m *CorruptGossipSubRouter) Protocols() []protocol.ID {
	return m.router.Protocols()
}

func (m *CorruptGossipSubRouter) Attach(sub *pubsub.PubSub) {
	m.router.Attach(sub)
}

func (m *CorruptGossipSubRouter) AddPeer(pid peer.ID, protocolId protocol.ID) {
	m.router.AddPeer(pid, protocolId)
}

func (m *CorruptGossipSubRouter) RemovePeer(pid peer.ID) {
	m.router.RemovePeer(pid)
}

func (m *CorruptGossipSubRouter) EnoughPeers(topic string, suggested int) bool {
	return m.router.EnoughPeers(topic, suggested)
}

func (m *CorruptGossipSubRouter) AcceptFrom(pid peer.ID) pubsub.AcceptStatus {
	return m.router.AcceptFrom(pid)
}

func (m *CorruptGossipSubRouter) HandleRPC(rpc *pubsub.RPC) {
	m.router.HandleRPC(rpc)
}

func (m *CorruptGossipSubRouter) Publish(message *pubsub.Message) {
	m.router.Publish(message)
}

func (m *CorruptGossipSubRouter) Join(topic string) {
	m.router.Join(topic)
}

func (m *CorruptGossipSubRouter) Leave(topic string) {
	m.router.Leave(topic)
}

func (m *CorruptGossipSubRouter) SetPeerScore(score *pubsub.PeerScore) {
	m.router.SetPeerScore(score)
}

func (m *CorruptGossipSubRouter) GetPeerScore() *pubsub.PeerScore {
	return m.router.GetPeerScore()
}

func (m *CorruptGossipSubRouter) SetPeerScoreThresholds(thresholds *pubsub.PeerScoreThresholds) {
	m.router.SetPeerScoreThresholds(thresholds)
}

func (m *CorruptGossipSubRouter) SetGossipTracer(tracer *pubsub.GossipTracer) {
	m.router.SetGossipTracer(tracer)
}

func (m *CorruptGossipSubRouter) GetGossipTracer() *pubsub.GossipTracer {
	return m.router.GetGossipTracer()
}

func (m *CorruptGossipSubRouter) GetTagTracer() *pubsub.TagTracer {
	return m.router.GetTagTracer()
}

func (m *CorruptGossipSubRouter) SetDirectPeers(direct map[peer.ID]struct{}) {
	m.router.SetDirectPeers(direct)
}

func (m *CorruptGossipSubRouter) SetPeerGater(gater *pubsub.PeerGater) {
	m.router.SetPeerGater(gater)
}

func (m *CorruptGossipSubRouter) GetPeerGater() *pubsub.PeerGater {
	return m.router.GetPeerGater()
}
