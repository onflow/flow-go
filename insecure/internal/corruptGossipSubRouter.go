package internal

import (
	"fmt"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

// CorruptGossipSubRouter is a wrapper around GossipSubRouter that allows us to access the internal
// fields of the router for BFT testing and attack implementations.
type CorruptGossipSubRouter struct {
	router *corrupt.GossipSubRouter
}

var _ corrupt.GossipPubSubRouter = (*CorruptGossipSubRouter)(nil)

func NewCorruptGossipSubRouter(router *corrupt.GossipSubRouter) *CorruptGossipSubRouter {
	return &CorruptGossipSubRouter{
		router: router,
	}
}

func (m *CorruptGossipSubRouter) Protocols() []protocol.ID {
	return m.router.Protocols()
}

func (m *CorruptGossipSubRouter) Attach(sub *corrupt.PubSub) {
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

func (m *CorruptGossipSubRouter) AcceptFrom(pid peer.ID) corrupt.AcceptStatus {
	return m.router.AcceptFrom(pid)
}

func (m *CorruptGossipSubRouter) HandleRPC(rpc *corrupt.RPC) {
	fmt.Println("HandleRPC called: ", rpc.String())
	m.router.HandleRPC(rpc)
}

func (m *CorruptGossipSubRouter) Publish(message *corrupt.Message) {
	m.router.Publish(message)
}

func (m *CorruptGossipSubRouter) Join(topic string) {
	m.router.Join(topic)
}

func (m *CorruptGossipSubRouter) Leave(topic string) {
	m.router.Leave(topic)
}

func (m *CorruptGossipSubRouter) SetPeerScore(score *corrupt.PeerScore) {
	m.router.SetPeerScore(score)
}

func (m *CorruptGossipSubRouter) GetPeerScore() *corrupt.PeerScore {
	return m.router.GetPeerScore()
}

func (m *CorruptGossipSubRouter) SetPeerScoreThresholds(thresholds *corrupt.PeerScoreThresholds) {
	m.router.SetPeerScoreThresholds(thresholds)
}

func (m *CorruptGossipSubRouter) SetGossipTracer(tracer *corrupt.GossipTracer) {
	m.router.SetGossipTracer(tracer)
}

func (m *CorruptGossipSubRouter) GetGossipTracer() *corrupt.GossipTracer {
	return m.router.GetGossipTracer()
}

func (m *CorruptGossipSubRouter) GetTagTracer() *corrupt.TagTracer {
	return m.router.GetTagTracer()
}

func (m *CorruptGossipSubRouter) SetDirectPeers(direct map[peer.ID]struct{}) {
	m.router.SetDirectPeers(direct)
}

func (m *CorruptGossipSubRouter) SetPeerGater(gater *corrupt.PeerGater) {
	m.router.SetPeerGater(gater)
}

func (m *CorruptGossipSubRouter) GetPeerGater() *corrupt.PeerGater {
	return m.router.GetPeerGater()
}

func (m *CorruptGossipSubRouter) SendControl(p peer.ID, ctl *pb.ControlMessage) {
	fmt.Println("SendControl called: ", ctl.String())

	m.router.SendControl(p, ctl)
}
