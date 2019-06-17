package crypto

import (
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

type Account struct {
	PublicKey string
	secretKey string
	mnemonic  string
}

func GenKeypair(passphrase string) Account {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(256)
	mnemonic, _ := bip39.NewMnemonic(entropy)

	// Generate a Bip32 HD wallet for the mnemonic and a user supplied password
	seed := bip39.NewSeed(mnemonic, passphrase)

	masterKey, _ := bip32.NewMasterKey(seed)
	publicKey := masterKey.PublicKey()

	newAccount := Account{
		PublicKey: publicKey.String(),
		secretKey: masterKey.String(),
		mnemonic:  mnemonic,
	}

	return newAccount
}
