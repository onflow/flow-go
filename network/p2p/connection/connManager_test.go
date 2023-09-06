package connection_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	protectF     = "protect"
	unprotectF   = "unprotect"
	isProtectedF = "isprotected"
)

type fun struct {
	funName     string
	expectation bool
}

var protect = fun{
	protectF,
	false,
}
var unprotect = fun{
	unprotectF,
	false,
}
var isProtected = fun{
	isProtectedF,
	true,
}
var isNotProtected = fun{
	isProtectedF,
	false,
}

// TestConnectionManagerProtection tests that multiple protected and unprotected calls result in the correct IsProtected
// status for a peer ID
func TestConnectionManagerProtection(t *testing.T) {

	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	flowConfig, err := config.DefaultConfig()
	require.NoError(t, err)
	noopMetrics := metrics.NewNoopCollector()
	connManager, err := connection.NewConnManager(log, noopMetrics, &flowConfig.NetworkConfig.ConnectionManagerConfig)
	require.NoError(t, err)

	testCases := [][]fun{
		// single stream created on a connection
		{protect, isProtected, unprotect, isNotProtected},
		// two streams created on a connection at the same time
		{protect, protect, unprotect, isNotProtected, unprotect, isNotProtected},
		// two streams created on a connection one after another
		{protect, unprotect, isNotProtected, protect, unprotect, isNotProtected},
	}

	for _, testCase := range testCases {
		testSequence(t, testCase, connManager)
	}
}

func testSequence(t *testing.T, sequence []fun, connMgr *connection.ConnManager) {
	pID := generatePeerInfo(t)
	for _, s := range sequence {
		switch s.funName {
		case protectF:
			connMgr.Protect(pID, "global")
		case unprotectF:
			connMgr.Unprotect(pID, "global")
		case isProtectedF:
			require.Equal(t, connMgr.IsProtected(pID, ""), s.expectation, fmt.Sprintf("failed sequence: %v", sequence))
		}
	}
}

func generatePeerInfo(t *testing.T) peer.ID {
	key := p2pfixtures.NetworkingKeyFixtures(t)
	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))
	pInfo, err := utils.PeerAddressInfo(*identity)
	require.NoError(t, err)
	return pInfo.ID
}

// TestConnectionManager_Watermarking tests that the connection manager prunes connections when the number of connections
// exceeds the high watermark and that it does not prune connections when the number of connections is below the low watermark.
func TestConnectionManager_Watermarking(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()

	cfg := &netconf.ConnectionManagerConfig{
		HighWatermark: 4,                      // whenever the number of connections exceeds 4, connection manager prune connections.
		LowWatermark:  2,                      // connection manager prune connections until the number of connections is 2.
		GracePeriod:   500 * time.Millisecond, // extra connections will be pruned if they are older than a second (just for testing).
		SilencePeriod: time.Second,            // connection manager prune checking kicks in every 5 seconds (just for testing).
	}
	thisConnMgr, err := connection.NewConnManager(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		cfg)
	require.NoError(t, err)
	idProvider := unittest.NewUpdatableIDProvider(flow.IdentityList{})
	thisNode, identity := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithConnectionManager(thisConnMgr))
	idProvider.SetIdentities(flow.IdentityList{&identity})

	otherNodes, _ := p2ptest.NodesFixture(t, sporkId, t.Name(), 5, idProvider)

	nodes := append(otherNodes, thisNode)

	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	// connect this node to all other nodes.
	for _, otherNode := range otherNodes {
		require.NoError(t, thisNode.Host().Connect(ctx, otherNode.Host().Peerstore().PeerInfo(otherNode.ID())))
	}

	// ensures this node is connected to all other nodes (based on the number of connections).
	require.Eventuallyf(t, func() bool {
		return len(thisNode.Host().Network().Conns()) == len(otherNodes)
	}, 1*time.Second, 100*time.Millisecond, "expected %d connections, got %d", len(otherNodes), len(thisNode.Host().Network().Conns()))

	// wait for grace period to expire and connection manager kick in as the number of connections is beyond high watermark.
	time.Sleep(time.Second)

	// ensures that eventually connection manager closes connections till the low watermark is reached.
	require.Eventuallyf(t, func() bool {
		return len(thisNode.Host().Network().Conns()) == cfg.LowWatermark
	}, 1*time.Second, 100*time.Millisecond, "expected %d connections, got %d", cfg.LowWatermark, len(thisNode.Host().Network().Conns()))

	// connects this node to one of the other nodes that is pruned by connection manager.
	for _, otherNode := range otherNodes {
		if len(thisNode.Host().Network().ConnsToPeer(otherNode.ID())) == 0 {
			require.NoError(t, thisNode.Host().Connect(ctx, otherNode.Host().Peerstore().PeerInfo(otherNode.ID())))
			break // we only need to connect to one node.
		}
	}

	// wait for another grace period to expire and connection manager kick in.
	time.Sleep(time.Second)

	// ensures that connection manager does not close any connections as the number of connections is below low watermark.
	require.Eventuallyf(t, func() bool {
		return len(thisNode.Host().Network().Conns()) == cfg.LowWatermark+1
	}, 1*time.Second, 100*time.Millisecond, "expected %d connections, got %d", cfg.LowWatermark+1, len(thisNode.Host().Network().Conns()))
}
