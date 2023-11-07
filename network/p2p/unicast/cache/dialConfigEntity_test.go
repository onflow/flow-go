package unicastcache_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/unicast"
	unicastcache "github.com/onflow/flow-go/network/p2p/unicast/cache"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDialConfigEntity tests the DialConfigEntity struct and its methods.
func TestDialConfigEntity(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)

	d := &unicastcache.DialConfigEntity{
		PeerId: peerID,
		DialConfig: unicast.DialConfig{
			DialRetryAttemptBudget:           10,
			StreamCreationRetryAttemptBudget: 20,
			LastSuccessfulDial:               time.Now(),
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
		d2 := &unicastcache.DialConfigEntity{
			PeerId:     unittest.PeerIdFixture(t),
			DialConfig: d.DialConfig,
		}
		require.NotEqual(t, d.ID(), d2.ID()) // different peer id, different id.

		d3 := &unicastcache.DialConfigEntity{
			PeerId: d.PeerId,
			DialConfig: unicast.DialConfig{
				DialRetryAttemptBudget:           100,
				StreamCreationRetryAttemptBudget: 200,
				LastSuccessfulDial:               time.Now(),
			},
		}
		require.Equal(t, d.ID(), d3.ID()) // same peer id, same id, even though the dial config is different.
	})
}
