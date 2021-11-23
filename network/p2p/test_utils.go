package p2p

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

func createNode(
	t *testing.T,
	nodeID flow.Identifier,
	networkKey crypto.PrivateKey,
	sporkID flow.Identifier,
	psOpts ...PubsubOption,
) *Node {
	if len(psOpts) == 0 {
		psOpts = DefaultPubsubOptions(DefaultMaxPubSubMsgSize)
	}
	libp2pNode, err := NewDefaultLibP2PNodeBuilder(nodeID, "0.0.0.0:0", networkKey).
		SetSporkID(sporkID).
		SetPubsubOptions(psOpts...).
		SetStreamCompressor(WithGzipCompression).
		Build(context.TODO())
	require.NoError(t, err)

	return libp2pNode
}
