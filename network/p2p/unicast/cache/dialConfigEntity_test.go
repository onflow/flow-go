package unicastcache_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/network/p2p/unicast/unicastmodel"
)

// TestDialConfigEntity tests the DialConfigEntity struct and its methods.
func TestDialConfigEntity(t *testing.T) {
	peerID := p2ptest.PeerIdFixture(t)

	d := &unicastcache.DialConfigEntity{
		PeerId: peerID,
		DialConfig: unicastmodel.DialConfig{
			DialBackoff:        10,
			StreamBackoff:      20,
			LastSuccessfulDial: 30,
		},
	}

	t.Run("Test ID and Checksum", func(t *testing.T) {
		// id and checksum methods must return the same value as expected.
		expectedID := unicastcache.PeerIdToFlowId(peerID)
		require.Equal(t, expectedID, d.ID())
		require.Equal(t, expectedID, d.Checksum())
	})
}
