package unicastcache_test

import (
	"testing"
	"time"

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
			DialBackoffBudget:           10,
			StreamBackBudget:            20,
			LastSuccessfulDial:          time.Now(),
			ConsecutiveSuccessfulStream: 30,
		},
	}

	t.Run("Test ID and Checksum", func(t *testing.T) {
		// id and checksum methods must return the same value as expected.
		expectedID := unicastcache.PeerIdToFlowId(peerID)
		require.Equal(t, expectedID, d.ID())
		require.Equal(t, expectedID, d.Checksum())

		// id and checksum methods must always return the same value.
		require.Equal(t, expectedID, d.ID())
		require.Equal(t, expectedID, d.Checksum())
	})
}
