package crypto

import (
	"log"
	
	"golang.org/x/crypto/sha3"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// KeyPair represents a BIP32 public key and private key pair (and the seed phrase used to derive it).
type KeyPair struct {
	PublicKey string
	secretKey string
	mnemonic  string
}

// genKeyPair generates a new HD wallet keypair to be used for account creation.
func genKeyPair(passphrase string) *KeyPair {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, passphrase)

	masterKey, err := bip32.NewMasterKey(seed)

	if err != nil {
		log.Fatal(err)
	}

	publicKey := masterKey.PublicKey()

	newKeyPair := &KeyPair{
		PublicKey: publicKey.String(),
		secretKey: masterKey.String(),
		mnemonic:  mnemonic,
	}

	return newKeyPair
}

// ComputeHash computes the SHA3-256 hash of some arbitrary set of data.
func ComputeHash(data []byte) []byte {
	hash := sha3.New256()
	hash.Write(data)
	return hash.Sum(nil)
}
