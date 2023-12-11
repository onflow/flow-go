package unicastcache_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/unicast"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestUnicastConfigEntity tests the UnicastConfigEntity struct and its methods.
func TestUnicastConfigEntity(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)

	d := &unicastcache.UnicastConfigEntity{
		PeerId: peerID,
		Config: unicast.Config{
			StreamCreationRetryAttemptBudget: 20,
			ConsecutiveSuccessfulStream:      30,
		},
	}

	t.Run(
		"Test ID and Checksum", func(t *testing.T) {
			// id and checksum methods must return the same value as expected.
			expectedID := unicastcache.PeerIdToFlowId(peerID)
			require.Equal(t, expectedID, d.ID())
			require.Equal(t, expectedID, d.Checksum())

			// id and checksum methods must always return the same value.
			require.Equal(t, expectedID, d.ID())
			require.Equal(t, expectedID, d.Checksum())
		},
	)

	t.Run("ID is only calculated from peer.ID", func(t *testing.T) {
		d2 := &unicastcache.UnicastConfigEntity{
			PeerId: unittest.PeerIdFixture(t),
			Config: d.Config,
		}
		require.NotEqual(t, d.ID(), d2.ID()) // different peer id, different id.

		d3 := &unicastcache.UnicastConfigEntity{
			PeerId: d.PeerId,
			Config: unicast.Config{
				StreamCreationRetryAttemptBudget: 200,
			},
		}
		require.Equal(t, d.ID(), d3.ID()) // same peer id, same id, even though the unicast config is different.
	})
}
