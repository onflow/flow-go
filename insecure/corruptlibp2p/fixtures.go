package corruptlibp2p

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsubpb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptInspectorFunc wraps a normal RPC inspector with a corrupt inspector func by translating corrupt.RPC -> pubsubpb.RPC
// before calling Inspect func.
func CorruptInspectorFunc(inspector p2p.GossipSubRPCInspector) func(id peer.ID, rpc *corrupt.RPC) error {
	return func(id peer.ID, rpc *corrupt.RPC) error {
		return inspector.Inspect(id, CorruptRPCToPubSubRPC(rpc))
	}
}

// CorruptRPCToPubSubRPC translates a corrupt.RPC -> pubsub.RPC
func CorruptRPCToPubSubRPC(rpc *corrupt.RPC) *pubsub.RPC {
	return &pubsub.RPC{
		RPC: pubsubpb.RPC{
			Subscriptions:        rpc.Subscriptions,
			Publish:              rpc.Publish,
			Control:              rpc.Control,
			XXX_NoUnkeyedLiteral: rpc.XXX_NoUnkeyedLiteral,
			XXX_unrecognized:     rpc.XXX_unrecognized,
			XXX_sizecache:        rpc.XXX_sizecache,
		},
	}
}
