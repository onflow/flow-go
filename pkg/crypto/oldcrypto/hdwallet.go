package oldcrypto

import (
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

// Wallet is a wrapper around an HD wallet struct.
type Wallet struct {
	HDWallet *hdwallet.Wallet
	Mnemonic string
}

// CreateWallet creates a new HD wallet using a user-specified passphrase.
func CreateWallet(passphrase string) (*Wallet, error) {
	keypair, err := genKeyPair(passphrase)
	if err != nil {
		return nil, err
	}

	return GenWalletFromMnemonic(keypair.mnemonic)
}

// GenWalletFromMnemonic generates a HD wallet from an already known mnemonic.
func GenWalletFromMnemonic(mnemonic string) (*Wallet, error) {
	wallet, err := hdwallet.NewFromMnemonic(mnemonic)
	if err != nil {
		return nil, &InvalidMnemonic{mnemonic: mnemonic}
	}

	return &Wallet{
		HDWallet: wallet,
		Mnemonic: mnemonic,
	}, nil
}

// CreateAccount creates a new Bamboo user account.
func (w *Wallet) CreateAccount(publicKeys [][]byte, code []byte) (*Account, error) {
	return createAccountFromHDWallet(w.HDWallet, publicKeys, code, 0)
}

// CreateRootAccount creates a new Bamboo root user account when the state is initialized.
func (w *Wallet) CreateRootAccount() (*Account, error) {
	return createAccountFromHDWallet(w.HDWallet, [][]byte{}, []byte{}, 100)
}
