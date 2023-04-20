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

// TestSaltedShuffle tests that shufflers initialized with the same peer ID return different shuffles each time.
// It is due to the fact that the seed is salted with a random salt.
func TestSaltedShuffle(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)

	s1, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	s2, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	peerIds := p2ptest.PeerIdSliceFixture(t, 10)

	shuffled1 := s1.Shuffle(peerIds)
	shuffled2 := s2.Shuffle(peerIds)

	// shuffle should not be in place, so shuffled1 and shuffle2 should not be equal to peerIds
	require.NotEqual(t, peerIds, shuffled1)
	require.NotEqual(t, peerIds, shuffled2)
	// however, element-wise it should be equal
	require.ElementsMatch(t, peerIds, shuffled1)
	require.ElementsMatch(t, peerIds, shuffled2)

	// since seeds are salted, even though the same peer id is used, the shuffle should be different
	require.NotEqual(t, shuffled1, shuffled2)

	// we keep shuffling for 10 times and make sure that shufflers are not returning the same shuffle.
	for i := 0; i < 10; i++ {
		shuffled1 = s1.Shuffle(peerIds)
		shuffled2 = s2.Shuffle(peerIds)
		require.NotEqual(t, shuffled1, shuffled2)
		require.ElementsMatch(t, peerIds, shuffled1)
		require.ElementsMatch(t, peerIds, shuffled2)
	}
}

// TestShuffleEmptySlice tests that shuffling an empty slice returns an empty slice.
func TestShuffleEmptySlice(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)

	s, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	peerIds := p2ptest.PeerIdSliceFixture(t, 0)

	shuffled := s.Shuffle(peerIds)

	require.Len(t, shuffled, 0)
}

// TestShuffleSingleElementSlice tests that shuffling a slice with a single element returns the same slice.
// This is because the shuffle is not performed if the slice has a single element.
func TestShuffleSingleElementSlice(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)

	s, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	peerIds := p2ptest.PeerIdSliceFixture(t, 1)

	shuffled := s.Shuffle(peerIds)

	require.Len(t, shuffled, 1)
	require.Equal(t, peerIds, shuffled)
}

// TestShuffleNilSlice tests that shuffling a nil slice returns an empty slice.
// This is because the shuffle is not performed if the slice is nil.
func TestShuffleNilSlice(t *testing.T) {
	pid := p2ptest.PeerIdFixture(t)

	s, err := connection.NewPeerIdSliceShuffler(pid)
	require.NoError(t, err)

	shuffled := s.Shuffle(nil)

	require.Len(t, shuffled, 0)
}
