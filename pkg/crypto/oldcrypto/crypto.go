package oldcrypto

import (
	"golang.org/x/crypto/sha3"

	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"
)

// KeyPair represents a BIP32 public key and private key pair (and the seed phrase used to derive it).
type KeyPair struct {
	PublicKey []byte
	secretKey []byte
	mnemonic  string
}

// Signature is a compound type combining a signature with an account address.
type Signature struct {
	Account Address
	Sig     []byte
}

// genKeyPair generates a new HD wallet keypair to be used for account creation.
func genKeyPair(passphrase string) (*KeyPair, error) {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, passphrase)

	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, &InvalidSeed{seed: string(seed)}
	}

	publicKey := masterKey.PublicKey()

	return &KeyPair{
		PublicKey: []byte(publicKey.String()),
		secretKey: []byte(masterKey.String()),
		mnemonic:  mnemonic,
	}, nil
}

// ComputeHash computes the SHA3-256 hash of some arbitrary set of data.
func ComputeHash(data []byte) []byte {
	hash := sha3.New256()
	hash.Write(data)
	return hash.Sum(nil)
}

// Sign signs a digest with the provided key pair.
func Sign(digest Hash, account Address, keyPair *KeyPair) *Signature {
	// TODO: implement real signatures
	return &Signature{
		Account: account,
		Sig:     nil,
	}
}
