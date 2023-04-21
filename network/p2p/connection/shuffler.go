package connection

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	mathrand "math/rand"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/flow"
)

// PeerIdSliceShuffler is a shuffler for peer ID slices.
// It is initialized with a seed derived from the given peer ID.
// The seed is generated deterministically from the peer ID, and is salted with a random salt.
// The salt is used to introduce randomness and uniqueness in the seed, and make the seed externally unpredictable.
// The shuffling is NOT done in place, rather a new deep copy of the slice is shuffled and returned.
// The shuffling is deterministic, and the same seed will always produce the same shuffling.
// The shuffling is done using the Fisher-Yates algorithm.
// The shuffling is done using a math/rand.Rand instance, which is NOT thread-safe.
type PeerIdSliceShuffler struct {
	// rng is the random number generator used for shuffling.
	// It is seeded with a salted seed derived from the given peer ID.
	rng *mathrand.Rand
}

// NewPeerIdSliceShuffler returns a new PeerIdSliceShuffler.
// The shuffler is initialized with a seed derived from the given peer ID.
// The seed is generated deterministically from the peer ID, and is salted with a random salt.
// The salt is used to introduce randomness and uniqueness in the seed, and make the seed externally unpredictable.
// Args:
//   - pid: the peer ID to generate the seed for
//
// Returns:
// - *PeerIdSliceShuffler: the shuffler
// - error: an error if the salt could not be generated. The error is irrecoverable.
func NewPeerIdSliceShuffler(pid peer.ID) (*PeerIdSliceShuffler, error) {
	seed, err := saltedSeedFromPeerID(pid)
	if err != nil {
		return nil, fmt.Errorf("could not generate salted seed: %w", err)
	}
	return &PeerIdSliceShuffler{
		rng: mathrand.New(mathrand.NewSource(int64(seed))),
	}, nil
}

// Shuffle shuffles the given peer ID slice.
// The shuffling is NOT done in place, rather a new deep copy of the slice is shuffled and returned.
// Args:
//   - peerIds: the peer ID slice to shuffle
//
// Returns:
//   - peer.IDSlice: the shuffled peer ID slice
func (s *PeerIdSliceShuffler) Shuffle(peerIds peer.IDSlice) peer.IDSlice {
	shuffled := make(peer.IDSlice, len(peerIds))
	copy(shuffled, peerIds)

	s.rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled
}

// saltedSeedFromPeerID returns a deterministic salted seed for the given peer ID.
// The salt is generated randomly. The salt is used to introduce randomness and uniqueness in the seed, and
// make the seed externally unpredictable.
// Args:
//   - peerID: the peer ID to generate the seed for
//
// Returns:
//   - seed: the salted seed
//   - error: an error if the salt could not be generated. The error is irrecoverable.
func saltedSeedFromPeerID(peerID peer.ID) (uint64, error) {
	salt := make([]byte, len(peerID))
	n, err := rand.Read(salt)
	if err != nil {
		return uint64(0), fmt.Errorf("could not generate random salt: %w", err)
	}
	if n != len(salt) {
		return uint64(0), fmt.Errorf("length of salt does not match. got: %d, expected: %d", n, len(salt))
	}

	seed := flow.MakeID(struct {
		Salt   []byte
		PeerID peer.ID
	}{
		Salt:   salt,
		PeerID: peerID,
	})

	return binary.BigEndian.Uint64(seed[:]), nil
}
