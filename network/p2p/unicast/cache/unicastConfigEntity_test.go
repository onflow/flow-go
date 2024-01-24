package unicastcache_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
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
		EntityId: flow.MakeID(peerID),
	}

	t.Run(
		"Test ID and Checksum", func(t *testing.T) {
			// id and checksum methods must return the same value as expected.
			expectedID := flow.MakeID(peerID)
			require.Equal(t, expectedID, d.ID())
			require.Equal(t, expectedID, d.Checksum())

			// id and checksum methods must always return the same value.
			require.Equal(t, expectedID, d.ID())
			require.Equal(t, expectedID, d.Checksum())
		},
	)

	t.Run("ID is only calculated from peer.ID", func(t *testing.T) {
		peerId := unittest.PeerIdFixture(t)
		d2 := &unicastcache.UnicastConfigEntity{
			PeerId:   peerId,
			Config:   d.Config,
			EntityId: flow.MakeID(peerId),
		}
		require.NotEqual(t, d.ID(), d2.ID()) // different peer id, different id.

		d3 := &unicastcache.UnicastConfigEntity{
			PeerId: d.PeerId,
			Config: unicast.Config{
				StreamCreationRetryAttemptBudget: 200,
			},
			EntityId: d.EntityId,
		}
		require.Equal(t, d.ID(), d3.ID()) // same peer id, same id, even though the unicast config is different.
	})
}
