package tracer

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// GossipSubNoopTracer is a no-op tracer that implements the RawTracer interface
// for the Flow network.
type GossipSubNoopTracer struct {
}

var _ pubsub.RawTracer = (*GossipSubNoopTracer)(nil)

func (t *GossipSubNoopTracer) AddPeer(p peer.ID, proto protocol.ID) {
	// no-op
}

func (t *GossipSubNoopTracer) RemovePeer(p peer.ID) {
	// no-op
}

func (t *GossipSubNoopTracer) Join(topic string) {
	// no-op
}

func (t *GossipSubNoopTracer) Leave(topic string) {
	// no-op
}

func (t *GossipSubNoopTracer) Graft(p peer.ID, topic string) {
	// no-op
}

func (t *GossipSubNoopTracer) Prune(p peer.ID, topic string) {
	// no-op
}

func (t *GossipSubNoopTracer) ValidateMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubNoopTracer) DeliverMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubNoopTracer) RejectMessage(msg *pubsub.Message, reason string) {
	// no-op
}

func (t *GossipSubNoopTracer) DuplicateMessage(msg *pubsub.Message) {
	// no-op
}

func (t *GossipSubNoopTracer) ThrottlePeer(p peer.ID) {
	// no-op
}

func (t *GossipSubNoopTracer) RecvRPC(rpc *pubsub.RPC) {
	// no-op
}

func (t *GossipSubNoopTracer) SendRPC(rpc *pubsub.RPC, p peer.ID) {
	// no-op
}

func (t *GossipSubNoopTracer) DropRPC(rpc *pubsub.RPC, p peer.ID) {
	// no-op
}

func (t *GossipSubNoopTracer) UndeliverableMessage(msg *pubsub.Message) {
	// no-op
}

func NewGossipSubNoopTracer() *GossipSubNoopTracer {
	return &GossipSubNoopTracer{}
}
