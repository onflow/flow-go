package p2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// DisallowListConsumer consumes notifications from the cache.NodeBlocklistWrapper whenever the block list is updated.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type DisallowListConsumer interface {
	// OnNodeBlockListUpdate notifications whenever the node block list is updated.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnNodeBlockListUpdate(list flow.IdentifierList)
}

// ControlMessageType is the type of control message, as defined in the libp2p pubsub spec.
type ControlMessageType string

const (
	CtrlMsgIHave ControlMessageType = "IHAVE"
	CtrlMsgIWant ControlMessageType = "IWANT"
	CtrlMsgGraft ControlMessageType = "GRAFT"
	CtrlMsgPrune ControlMessageType = "PRUNE"
)

// GossipSubRpcInspectorConsumer is the interface for a consumer of inspection result for GossipSub RPC messages.
// Implementations must:
//   - be concurrency safe
//   - be non-blocking
type GossipSubRpcInspectorConsumer interface {
	// OnInvalidControlMessage is called when a control message is received that is invalid according to the
	// Flow protocol specification.
	// The int parameter is the count of invalid messages received from the peer.
	// Prerequisites:
	// Implementation must be concurrency safe and non-blocking.
	OnInvalidControlMessage(peer.ID, ControlMessageType, uint64)
}
