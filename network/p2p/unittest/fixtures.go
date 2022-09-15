package unittest

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func PeerIdFixture(t *testing.T) peer.ID {
	id := unittest.IdentityFixture()
	_, _, key, err := p2p.NetworkingInfo(*id)
	require.NoError(t, err)

	peerID, err := peer.IDFromPublicKey(key)
	require.NoError(t, err)

	return peerID
}

func PeerIdsFixture(t *testing.T, n int) []peer.ID {
	peerIDs := make([]peer.ID, n)
	for i := 0; i < n; i++ {
		peerIDs[i] = PeerIdFixture(t)
	}
	return peerIDs
}
