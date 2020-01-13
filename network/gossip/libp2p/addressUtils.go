package libp2p

import (
	"crypto/md5"
	"encoding/binary"
	"io"
	"math/rand"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
)

// GetPeerID returns the LibP2P peer id derived from the given Name
// e.g. node1 will generate a peer id of QmUqrhCAbnT7jnhMnKY2d1Py9N5PfEvvHazuJfpzn5fFVB
func GetPeerID(name string) (peer.ID, error) {
	key, err := GetPublicKey(name)
	if err != nil {
		return "", err
	}
	id, err := peer.IDFromPublicKey(key.GetPublic())
	if err != nil {
		return "", err
	}
	return id, nil
}

// GetPublicKey generates a ECDSA key pair using the given seed
func GetPublicKey(seed string) (crypto.PrivKey, error) {
	s, err := generateSeed(seed)
	if err != nil {
		return nil, err
	}
	var r io.Reader = rand.New(rand.NewSource(s))
	// Creates a new ECDSA key pair for this host.
	// If RSA is used instead of ECDSA, the keys are not generated in a deterministic manner since GoLang RSA
	// implementation is not deterministic
	prvKey, _, err := crypto.GenerateKeyPairWithReader(crypto.ECDSA, 2048, r)
	return prvKey, err
}

// Generates an int64 seed given a string in a deterministic and consistent manner (the same seed string will always
// return the same int64 seed)
func generateSeed(seed string) (int64, error) {
	h := md5.New()
	// Generate the MD5 hash of the given string
	_, err := io.WriteString(h, seed)
	if err != nil {
		return 0, err
	}
	// Get the current hash
	s := binary.BigEndian.Uint64(h.Sum(nil))
	// Seed the random generator
	rand.Seed(int64(s))
	// Generate a pseudo-random number
	return rand.Int63(), nil
}
