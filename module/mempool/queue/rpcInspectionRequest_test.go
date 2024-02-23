package queue_test

import (
	"github.com/onflow/flow-go/module/mempool/queue"
	"testing"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestMessageEntity_InspectRPCRequest_ID tests that the ID of a MessageEntity created from an InspectRPCRequest is
// only dependent of the Nonce and PeerID fields of the InspectRPCRequest; and is independent of the RPC field.
// Unique identifier for the HeroCache is imperative to prevent false-positive de-duplication.
// However, the RPC field contains a bulk of the data in the InspectRPCRequest, and including it in the ID would
// cause the InspectRPCRequest store and retrieval to be resource intensive.
func TestMessageEntity_InspectRPCRequest_ID(t *testing.T) {
	rpcs := p2ptest.GossipSubRpcFixtures(t, 2)
	rpc1 := rpcs[0]
	rpc2 := rpcs[1]
	peerId1 := unittest.PeerIdFixture(t)

	// creates two InspectRPCRequest structs with the same Nonce and PeerID fields
	req1, err := validation.NewInspectRPCRequest(peerId1, &pubsub.RPC{
		RPC: *rpc1,
	})
	require.NoError(t, err)

	req2, err := validation.NewInspectRPCRequest(peerId1, &pubsub.RPC{
		RPC: *rpc1,
	})
	require.NoError(t, err)
	// Set the Nonce field of the second InspectRPCRequest struct to the Nonce field of the first
	req2.Nonce = req1.Nonce

	// creates a third InspectRPCRequest struct with the same Nonce and PeerID fields as the first two
	// but with a different RPC field
	req3, err := validation.NewInspectRPCRequest(peerId1, &pubsub.RPC{
		RPC: *rpc2,
	})
	require.NoError(t, err)
	req3.Nonce = req1.Nonce

	// now convert to MessageEntity
	entity1 := queue.NewMessageEntity(&engine.Message{Payload: req1})
	entity2 := queue.NewMessageEntity(&engine.Message{Payload: req2})
	entity3 := queue.NewMessageEntity(&engine.Message{Payload: req3})

	// as the Nonce and PeerID fields are the same, the ID of the MessageEntity should be the same accross all three
	// in other words, the RPC field should not affect the ID
	require.Equal(t, entity1.ID(), entity2.ID())
	require.Equal(t, entity1.ID(), entity3.ID())
}
