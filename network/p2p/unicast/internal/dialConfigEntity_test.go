package internal_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/unicast/internal"
)

// TestDialConfigEntity tests the DialConfigEntity struct and its methods.
func TestDialConfigEntity(t *testing.T) {
	peerID := p2ptest.PeerIdFixture(t)

	d := &internal.DialConfigEntity{
		PeerId:             peerID,
		DialBackoff:        10,
		StreamBackoff:      20,
		LastSuccessfulDial: 30,
	}

	t.Run("Test ID and Checksum", func(t *testing.T) {
		// id and checksum methods must return the same value as expected.
		expectedID := flow.MakeIDFromFingerPrint([]byte(d.PeerId))
		require.Equal(t, expectedID, d.ID())
		require.Equal(t, expectedID, d.Checksum())
	})
}
