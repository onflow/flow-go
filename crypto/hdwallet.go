package crypto

import (
	"log"

	"github.com/miguelmota/go-ethereum-hdwallet"
)

type Wallet struct {
	HDWallet	*hdwallet.Wallet
}

// CreateWallet creates a new HD wallet using a user-specified passphrase.
func CreateWallet(passphrase string) *Wallet {
	keypair := genKeyPair(passphrase)
	wallet, err := hdwallet.NewFromMnemonic(keypair.mnemonic)
	
	if err != nil {
		log.Fatal(err)
	}

	return &Wallet{
		HDWallet: wallet,
	}
}

// CreateAccount creates a new Bamboo user account.
func (w *Wallet) CreateAccount(publicKeys [][]byte, code []byte) *Account {
	return createAccountFromHDWallet(w.HDWallet, publicKeys, code)
}
