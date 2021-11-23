package p2p

import (
	"fmt"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

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

// TestConnectionManagerProtection tests that multiple protect and unprotect calls result in the correct IsProtected
// status for a peer ID
func TestConnectionManagerProtection(t *testing.T) {

	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	noopMetrics := metrics.NewNoopCollector()
	connManager := NewConnManager(log, noopMetrics)

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

func testSequence(t *testing.T, sequence []fun, connMgr *ConnManager) {
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
	key := generateNetworkingKey(t)
	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))
	pInfo, err := PeerAddressInfo(*identity)
	require.NoError(t, err)
	return pInfo.ID
}
