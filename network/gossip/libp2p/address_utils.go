package libp2p

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/multiformats/go-multiaddr"
)

// GetLocationMultiaddr returns a Multiaddress (https://docs.libp2p.io/concepts/addressing/) given a node address
func GetLocationMultiaddr(id NodeAddress) (multiaddr.Multiaddr, error) {
	return multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%s", id.ip, id.port))
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
