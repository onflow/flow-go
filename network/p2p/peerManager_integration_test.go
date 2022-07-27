package p2p_test

import (
	"context"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestPeerManager_Integration(t *testing.T) {
	count := 5
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create nodes
	nodes, identities := nodesFixture(t, ctx, unittest.IdentifierFixture(), "test_peer_manager", count)
	defer stopNodes(t, nodes)

	thisNode := nodes[0]
	thisId := identities[0]
	othersId := identities[1:]

	connector, err := p2p.NewLibp2pConnector(unittest.Logger(), thisNode.Host(), p2p.ConnectionPruningEnabled)
	require.NoError(t, err)

	peerManager := p2p.NewPeerManager(unittest.Logger(), func() peer.IDSlice {

	})
}
