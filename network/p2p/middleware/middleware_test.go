package middleware_test

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestChunkDataPackMaxMessageSize tests that the max message size for a chunk data pack response is set to the large message size.
func TestChunkDataPackMaxMessageSize(t *testing.T) {
	// creates an outgoing chunk data pack response message (imitating an EN is sending a chunk data pack response to VN).
	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{unittest.IdentifierFixture()},
		channels.ProvideChunks,
		&messages.ChunkDataResponse{
			ChunkDataPack: *unittest.ChunkDataPackFixture(unittest.IdentifierFixture()),
			Nonce:         rand.Uint64(),
		},
		unittest.NetworkCodec().Encode,
		message.ProtocolTypeUnicast)
	require.NoError(t, err)

	// get the max message size for the message
	size, err := middleware.UnicastMaxMsgSizeByCode(msg.Proto().Payload)
	require.NoError(t, err)
	require.Equal(t, middleware.LargeMsgMaxUnicastMsgSize, size)
}
