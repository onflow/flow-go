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

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/internal/p2pfixtures"
	"github.com/onflow/flow-go/network/p2p/connection"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/utils"

	"github.com/onflow/flow-go/module/metrics"
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
	noopMetrics := metrics.NewNoopCollector()
	connManager, err := connection.NewConnManager(log, noopMetrics, connection.DefaultConnManagerConfig())
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

func TestConnectionManager_Watermarking(t *testing.T) {
	sporkId := unittest.IdentifierFixture()
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	defer cancel()

	thisConnMgr, err := connection.NewConnManager(
		unittest.Logger(),
		metrics.NewNoopCollector(),
		&connection.ManagerConfig{
			HighWatermark: 4,
			LowWatermark:  2,
		})
	require.NoError(t, err)

	thisNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithConnectionManager(thisConnMgr))

	otherNodes, _ := p2ptest.NodesFixture(t, sporkId, t.Name(), 5)

	nodes := append(otherNodes, thisNode)

	p2ptest.StartNodes(t, signalerCtx, nodes, 100*time.Millisecond)
	defer p2ptest.StopNodes(t, nodes, cancel, 100*time.Millisecond)

	// connect this node to all other nodes
	for _, otherNode := range otherNodes {
		require.NoError(t, thisNode.Host().Connect(ctx, otherNode.Host().Peerstore().PeerInfo(otherNode.Host().ID())))
	}

	time.Sleep(20 * time.Second)

	fmt.Println(len(thisNode.Host().Network().Conns()))
}
