package p2p

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProtectUnprotect tests that ProtectPeer marks a peer as protected and UnprotectPeer marks it as unprotected
func TestProtectUnprotect(t *testing.T) {
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	noopMetrics := metrics.NewNoopCollector()

	connManager := NewConnManager(log, noopMetrics)

	key := generateNetworkingKey(t)
	identity := unittest.IdentityFixture(unittest.WithNetworkingKey(key.PublicKey()), unittest.WithAddress("1.1.1.1:0"))
	pInfo, err := PeerAddressInfo(*identity)
	require.NoError(t, err)

	connManager.ProtectPeer(pInfo.ID)
	require.True(t, connManager.IsProtected(pInfo.ID, ""))
	connManager.UnprotectPeer(pInfo.ID)
	require.False(t, connManager.IsProtected(pInfo.ID, ""))
}
