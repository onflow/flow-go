package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	bip32 "github.com/tyler-smith/go-bip32"
	bip39 "github.com/tyler-smith/go-bip39"

	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type AccountConfig struct {
	Address string `json:"account"`
	Seed    string `json:"seed"`
}

type AccountKey struct {
	Account    types.Address
	PublicKey  []byte
	PrivateKey []byte
}

type KeyPair struct {
	PublicKey  []byte
	PrivateKey []byte
}

func LoadAccountFromFile(filename string) (*AccountKey, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	return LoadAccount(f)
}

func LoadAccount(r io.Reader) (*AccountKey, error) {
	d := json.NewDecoder(r)

	var conf AccountConfig

	if err := d.Decode(&conf); err != nil {
		return nil, err
	}

	keyPair, err := GenerateKeyPair(conf.Seed)
	if err != nil {
		return nil, err
	}

	return &AccountKey{
		Account:    types.HexToAddress(conf.Address),
		PublicKey:  keyPair.PublicKey,
		PrivateKey: keyPair.PrivateKey,
	}, nil
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
		return nil, fmt.Errorf("invalid seed phrase")
	}

	publicKey := masterKey.PublicKey()

	publicKeyBytes, _ := publicKey.Serialize()
	privateKeyBytes, _ := masterKey.Serialize()

	return &KeyPair{
		PublicKey:  publicKeyBytes,
		PrivateKey: privateKeyBytes,
	}, nil
}
