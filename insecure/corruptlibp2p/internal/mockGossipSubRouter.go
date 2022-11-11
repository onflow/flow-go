package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

type GossipSubRouterFixture struct {
	router *pubsub.GossipSubRouter
}

var _ pubsub.GossipPubSubRouter = (*GossipSubRouterFixture)(nil)

func NewGossipSubRouterFixture() *GossipSubRouterFixture {
	return &GossipSubRouterFixture{
		router: pubsub.DefaultGossipSubRouter(),
	}
}

func (m *GossipSubRouterFixture) Protocols() []protocol.ID {
	return m.router.Protocols()
}

func (m *GossipSubRouterFixture) Attach(sub *pubsub.PubSub) {
	m.router.Attach(sub)
}

func (m *GossipSubRouterFixture) AddPeer(pid peer.ID, protocolId protocol.ID) {
	m.AddPeer(pid, protocolId)
}

func (m *GossipSubRouterFixture) RemovePeer(pid peer.ID) {
	m.RemovePeer(pid)
}

func (m *GossipSubRouterFixture) EnoughPeers(topic string, suggested int) bool {
	return m.router.EnoughPeers(topic, suggested)
}

func (m *GossipSubRouterFixture) AcceptFrom(pid peer.ID) pubsub.AcceptStatus {
	return m.router.AcceptFrom(pid)
}

func (m *GossipSubRouterFixture) HandleRPC(rpc *pubsub.RPC) {
	m.router.HandleRPC(rpc)
}

func (m *GossipSubRouterFixture) Publish(message *pubsub.Message) {
	m.router.Publish(message)
}

func (m *GossipSubRouterFixture) Join(topic string) {
	m.router.Join(topic)
}

func (m *GossipSubRouterFixture) Leave(topic string) {
	m.router.Leave(topic)
}

func (m *GossipSubRouterFixture) SetPeerScore(score *pubsub.PeerScore) {
	m.router.SetPeerScore(score)
}

func (m *GossipSubRouterFixture) GetPeerScore() *pubsub.PeerScore {
	return m.router.GetPeerScore()
}

func (m *GossipSubRouterFixture) SetPeerScoreThresholds(thresholds *pubsub.PeerScoreThresholds) {
	m.router.SetPeerScoreThresholds(thresholds)
}

func (m *GossipSubRouterFixture) SetGossipTracer(tracer *pubsub.GossipTracer) {
	m.router.SetGossipTracer(tracer)
}

func (m *GossipSubRouterFixture) GetGossipTracer() *pubsub.GossipTracer {
	return m.router.GetGossipTracer()
}

func (m *GossipSubRouterFixture) GetTagTracer() *pubsub.TagTracer {
	return m.router.GetTagTracer()
}

func (m *GossipSubRouterFixture) SetDirectPeers(direct map[peer.ID]struct{}) {
	m.router.SetDirectPeers(direct)
}

func (m *GossipSubRouterFixture) SetPeerGater(gater *pubsub.PeerGater) {
	m.router.SetPeerGater(gater)
}

func (m *GossipSubRouterFixture) GetPeerGater() *pubsub.PeerGater {
	return m.router.GetPeerGater()
}
