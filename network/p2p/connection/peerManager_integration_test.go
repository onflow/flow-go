package connection_test

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPeerManager_Integration tests the correctness of integration between PeerManager and PeerUpdater over
// a fully connected topology.
// PeerManager should be able to connect to all peers using the connector, and must also tear down the connection to
// peers that are excluded from its identity provider.
func TestPeerManager_Integration(t *testing.T) {
	count := 5
	ctx, cancel := context.WithCancel(context.Background())

	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)

	// create nodes
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	nodes, identities := p2ptest.NodesFixture(t, unittest.IdentifierFixture(), "test_peer_manager", count, idProvider)
	idProvider.SetIdentities(identities)
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	thisNode := nodes[0]
	topologyPeers := identities[1:]

	// adds address of all other nodes into the peer store of this node, so that it can dial them.
	info, invalid := utils.PeerInfosFromIDs(topologyPeers)
	require.Empty(t, invalid)
	for _, i := range info {
		thisNode.Host().Peerstore().SetAddrs(i.ID, i.Addrs, peerstore.PermanentAddrTTL)
	}

	connector, err := connection.DefaultLibp2pBackoffConnectorFactory()(thisNode.Host())
	require.NoError(t, err)
	// setup
	peerUpdater, err := connection.NewPeerUpdater(&connection.PeerUpdaterConfig{
		PruneConnections: connection.PruningEnabled,
		Logger:           unittest.Logger(),
		Host:             connection.NewConnectorHost(thisNode.Host()),
		Connector:        connector,
	})
	require.NoError(t, err)

	idTranslator, err := translator.NewFixedTableIdentityTranslator(identities)
	require.NoError(t, err)

	peerManager := connection.NewPeerManager(unittest.Logger(), connection.DefaultPeerUpdateInterval, peerUpdater)
	peerManager.SetPeersProvider(func() peer.IDSlice {
		// peerManager is furnished with a full topology that connects to all nodes
		// in the topologyPeers.
		peers := peer.IDSlice{}
		for _, id := range topologyPeers {
			peerId, err := idTranslator.GetPeerID(id.NodeID)
			require.NoError(t, err)
			peers = append(peers, peerId)
		}

		return peers
	})

	// initially no node should be in peer store of this node.
	require.Empty(t, thisNode.Host().Network().Peers())
	peerManager.ForceUpdatePeers(ctx)
	time.Sleep(1 * time.Second)
	// after a forced update all other nodes must be added to the peer store of this node.
	require.Len(t, thisNode.Host().Network().Peers(), count-1)
	// after a forced update there must be a connection between this node and other nodes
	connectedToAll(t, thisNode.Host(), idTranslator, topologyPeers.NodeIDs())

	// kicks one node out of the othersIds; this imitates evicting, ejecting, or unstaking a node
	evictedId := topologyPeers[0]     // evicted one
	topologyPeers = topologyPeers[1:] // updates otherIds list
	peerManager.ForceUpdatePeers(ctx)
	time.Sleep(1 * time.Second)
	// after a forced update, the evicted one should be excluded from the peer store.
	require.Len(t, thisNode.Host().Network().Peers(), count-2)
	// there must be a connection between this node and other nodes (except evicted one).
	connectedToAll(t, thisNode.Host(), idTranslator, topologyPeers.NodeIDs())

	// there must be no connection between this node and evicted one
	peerId, err := idTranslator.GetPeerID(evictedId.NodeID)
	require.NoError(t, err)
	assert.Equal(t, thisNode.Host().Network().Connectedness(peerId), network.NotConnected)
}

// connectedToAll is a test helper that fails if there is no connection between this host and at least one of the
// nodes in "all".
func connectedToAll(t *testing.T, host host.Host, translator p2p.IDTranslator, all flow.IdentifierList) {
	for _, id := range all {
		peerId, err := translator.GetPeerID(id)
		require.NoError(t, err)
		assert.Equal(t, host.Network().Connectedness(peerId), network.Connected)
	}
}
