package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

type GossipSubRouterFixture struct {
	Router *pubsub.GossipSubRouter
}

var _ pubsub.GossipPubSubRouter = (*GossipSubRouterFixture)(nil)

func NewGossipSubRouterFixture() *GossipSubRouterFixture {
	return &GossipSubRouterFixture{
		Router: pubsub.DefaultGossipSubRouter(),
	}
}

func (m *GossipSubRouterFixture) Protocols() []protocol.ID {
	return m.Router.Protocols()
}

func (m *GossipSubRouterFixture) Attach(sub *pubsub.PubSub) {
	m.Router.Attach(sub)
}

func (m *GossipSubRouterFixture) AddPeer(pid peer.ID, protocolId protocol.ID) {
	m.AddPeer(pid, protocolId)
}

func (m *GossipSubRouterFixture) RemovePeer(pid peer.ID) {
	m.RemovePeer(pid)
}

func (m *GossipSubRouterFixture) EnoughPeers(topic string, suggested int) bool {
	return m.Router.EnoughPeers(topic, suggested)
}

func (m *GossipSubRouterFixture) AcceptFrom(pid peer.ID) pubsub.AcceptStatus {
	return m.Router.AcceptFrom(pid)
}

func (m *GossipSubRouterFixture) HandleRPC(rpc *pubsub.RPC) {
	m.Router.HandleRPC(rpc)
}

func (m *GossipSubRouterFixture) Publish(message *pubsub.Message) {
	m.Router.Publish(message)
}

func (m *GossipSubRouterFixture) Join(topic string) {
	m.Router.Join(topic)
}

func (m *GossipSubRouterFixture) Leave(topic string) {
	m.Router.Leave(topic)
}

func (m *GossipSubRouterFixture) SetPeerScore(score *pubsub.PeerScore) {
	m.Router.SetPeerScore(score)
}

func (m *GossipSubRouterFixture) GetPeerScore() *pubsub.PeerScore {
	return m.Router.GetPeerScore()
}

func (m *GossipSubRouterFixture) SetPeerScoreThresholds(thresholds *pubsub.PeerScoreThresholds) {
	m.Router.SetPeerScoreThresholds(thresholds)
}

func (m *GossipSubRouterFixture) SetGossipTracer(tracer *pubsub.GossipTracer) {
	m.Router.SetGossipTracer(tracer)
}

func (m *GossipSubRouterFixture) GetGossipTracer() *pubsub.GossipTracer {
	return m.Router.GetGossipTracer()
}

func (m *GossipSubRouterFixture) GetTagTracer() *pubsub.TagTracer {
	return m.Router.GetTagTracer()
}

func (m *GossipSubRouterFixture) SetDirectPeers(direct map[peer.ID]struct{}) {
	m.Router.SetDirectPeers(direct)
}

func (m *GossipSubRouterFixture) SetPeerGater(gater *pubsub.PeerGater) {
	m.Router.SetPeerGater(gater)
}

func (m *GossipSubRouterFixture) GetPeerGater() *pubsub.PeerGater {
	return m.Router.GetPeerGater()
}
