package connection_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/network/p2p/connection"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

// TestShuffle test that shuffle is not in place and that it returns a different shuffle each time
// it is called.
func TestShuffle(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)

	s, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	peerIds := p2ptest.PeerIdSliceFixture(t, 10)

	shuffled1 := s.Shuffle(peerIds)

	// shuffle should not be in place, so shuffled1 should not be equal to peerIds
	require.NotEqual(t, peerIds, shuffled1)
	// however, element-wise it should be equal
	require.ElementsMatch(t, peerIds, shuffled1)

	// next shuffle should be different than the previous one and should not be equal to peerIds
	shuffled2 := s.Shuffle(peerIds)
	require.NotEqual(t, peerIds, shuffled2)
	require.NotEqual(t, shuffled1, shuffled2)
	require.ElementsMatch(t, peerIds, shuffled2)
}
