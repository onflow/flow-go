package oldcrypto

import (
	bip32 "github.com/tyler-smith/go-bip32"
	"golang.org/x/crypto/sha3"
)

// KeyPair represents a BIP32 public/private key-pair.
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

// KeyPairFromSeed generates a BIP32 key-pair from a seed.
func KeyPairFromSeed(seed string) (*KeyPair, error) {
	masterKey, err := bip32.NewMasterKey([]byte(seed))
	if err != nil {
		return nil, &InvalidSeedError{seed}
	}

	publicKey := masterKey.PublicKey()

	publicKeyBytes, _ := publicKey.Serialize()
	privateKeyBytes, _ := masterKey.Serialize()

	return &KeyPair{
		PublicKey:  publicKeyBytes,
		PrivateKey: privateKeyBytes,
	}, nil
}

// ComputeHash computes the SHA3-256 hash of some arbitrary set of data.
func ComputeHash(data []byte) []byte {
	hash := sha3.New256()
	hash.Write(data)
	return hash.Sum(nil)
}
