package oldcrypto

import (
	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
	"golang.org/x/crypto/sha3"
)

// KeyPair represents a BIP32 public key and private key pair (and the seed phrase used to derive it).
type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

// GenerateKeyPair generates a new key-pair from a passphrase.
func GenerateKeyPair(passphrase string) (*KeyPair, error) {
	// generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, passphrase)

	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, &InvalidSeedError{seed: passphrase}
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
