package crypto

import (
	"log"
	"strconv"

	"github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

const (
	// Ethereum root path for reference
	rootPath = "m/44'/60'/0'/0/"
)

// Keypair represents a BIP32 public key and private key pair (and the seed phrase used to derive it).
type Keypair struct {
	PublicKey string
	secretKey string
	mnemonic  string
}

// Account represents an account on the Bamboo network (externally owned account or contract account w/ code).
type Account struct {
	Address    Address
	Balance    uint64
	Code       []byte
	PublicKeys [][]byte
	Path       string
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

// CreateAccount uses a specified HD wallet to create a new Bamboo user account.
func CreateAccount(w *hdwallet.Wallet) *Account {
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
	return &Account{
		Address: BytesToAddress(account.Address.Bytes()),
		Balance: 0,
		Code: []byte{},
		PublicKeys: [][]byte{publicKey},
		Path: path,
	}
}
