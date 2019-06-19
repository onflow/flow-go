package crypto

import (
	"log"
	"strconv"

	"github.com/miguelmota/go-ethereum-hdwallet"
)

const (
	// Ethereum root path for reference
	rootPath = "m/44'/60'/0'/0/"
)

// Account represents an account on the Bamboo network (externally owned account or contract account w/ code).
type Account struct {
	Address    Address
	Balance    uint64
	Code       []byte
	PublicKeys [][]byte
	Path       string
}

// createAccountFromHDWallet uses a specified HD wallet to create a new Bamboo user account.
func createAccountFromHDWallet(w *hdwallet.Wallet, publicKeys [][]byte, code []byte) *Account {
	index := len(w.Accounts())
	path := rootPath + strconv.Itoa(index)
	derivationPath := hdwallet.MustParseDerivationPath(path)
	account, err := w.Derive(derivationPath, true)
	
	if err != nil {
		log.Fatal(err)
	}

	publicKey, err := w.PublicKeyBytes(account)
	
	if err != nil {
		log.Fatal(err)
	}

	publicKeys = append([][]byte{publicKey}, publicKeys...)

	return &Account{
		Address: BytesToAddress(account.Address.Bytes()),
		Balance: 0,
		Code: code,
		PublicKeys: publicKeys,
		Path: path,
	}
}
