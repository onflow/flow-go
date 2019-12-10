package libp2p

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLibP2PNode_Start_Stop(t *testing.T) {
	nodes, err := createLibP2PNodes(t, 1)
	assert.NoError(t, err)
	assert.NoError(t, nodes[0].Stop())
}

// TestLibP2PNode_GetPeerInfo checks that given a node name, the corresponding node id is consistently generated
// e.g. Node name: "node1" always generates the libp2p node id QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N
func TestLibP2PNode_GetPeerInfo(t *testing.T) {
	var nodes []NodeAddress
	var ps []peer.AddrInfo
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			nodes = append(nodes, NodeAddress{name: fmt.Sprintf("node%d", j), ip: "1.1.1.1", port: "0"})
			p, err := GetPeerInfo(nodes[j])
			require.NoError(t, err)
			if i == 0 {
				ps = append(ps, p)
			} else {
				assert.Equal(t, ps[j].ID.String(), p.ID.String(), fmt.Sprintf(" Node ids not consistently generated"))
			}
		}
	}
}

// TestLibP2PNode_AddPeers checks if nodes can be added as peers to a given node
func TestLibP2PNode_AddPeers(t *testing.T) {
	var count = 10
	// Create 10 nodes
	nodes, err := createLibP2PNodes(t, count)
	require.NoError(t, err)
	defer func() {
		if nodes != nil {
			for _, n := range nodes {
				n.Stop()
			}
		}
	}()
	var ids []NodeAddress
	// Get actual ip and port numbers on which the nodes were started
	for _, n := range nodes[1:] {
		ip, p := n.GetIPPort()
		ids = append(ids, NodeAddress{name: n.name, ip: ip, port: p})
	}
	// To the 1st node add the remaining 9 nodes as peers.
	require.NoError(t, nodes[0].AddPeers(ids))
	assert.Eventuallyf(t, func() bool { return nodes[0].libP2PHost.Peerstore().Peers().Len() == count },
		2*time.Second, time.Millisecond,
		fmt.Sprintf("Peers expected: %d, found: %d", count-1, nodes[0].libP2PHost.Peerstore().Peers().Len()))
	fmt.Sprintf("Peers expected: %d, found: %d", count-1, nodes[0].libP2PHost.Peerstore().Peers().Len())

	// Check if libp2p reports the 9 nodes as connected with node 1.
	for _, a := range nodes[0].libP2PHost.Peerstore().Peers() {
		// A node is also a peer to itself but not marked as connected, hence skip checking that.
		if nodes[0].libP2PHost.ID().String() == a.String() {
			continue
		}
		assert.Equal(t, network.Connected, nodes[0].libP2PHost.Network().Connectedness(a))
	}
}

func createLibP2PNodes(t *testing.T, count int) (nodes []*P2PNode, err error) {
	defer func() {
		if err != nil && nodes != nil {
			for _, n := range nodes {
				n.Stop()
			}
		}
	}()
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	for i := 1; i <= count; i++ {
		var n = &P2PNode{}
		var nodeId = NodeAddress{name: fmt.Sprintf("node%d", i), ip: "0.0.0.0", port: "0"}
		err := n.Start(nodeId, l)
		if err != nil {
			break
		}
		nodes = append(nodes, n)
	}
	return nodes, err
}
