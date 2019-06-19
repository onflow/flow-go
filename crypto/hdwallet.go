package crypto

import (
	"log"

	"github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// Keypair represents a BIP32 public key and private key pair (and the seed phrase used to derive it).
type Keypair struct {
	PublicKey string
	secretKey string
	mnemonic  string
}

// genKeypair generates a new HD wallet keypair to be used for account creation.
func genKeypair(passphrase string) *Keypair {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, passphrase)

	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	newKeypair := &Keypair{
		PublicKey: publicKey.String(),
		secretKey: masterKey.String(),
		mnemonic:  mnemonic,
	}

	return newKeypair
}

// CreateWallet creates a new HD wallet using a user-specified passphrase.
func CreateWallet(passphrase string) *hdwallet.Wallet {
	keypair := genKeypair(passphrase)
	wallet, err := hdwallet.NewFromMnemonic(keypair.mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	return wallet
}
