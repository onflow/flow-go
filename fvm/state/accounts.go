package state

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	keyExists         = "exists"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

var ErrAccountNotFound = errors.New("account not found")

func keyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}

type Accounts struct {
	ledger    Ledger
	addresses *Addresses
}

func NewAccounts(ledger Ledger, addresses *Addresses) *Accounts {
	return &Accounts{
		ledger:    ledger,
		addresses: addresses,
	}
}

func (a *Accounts) Get(address flow.Address) (*flow.Account, error) {
	var ok bool
	var err error

	ok, err = a.Exists(address)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, ErrAccountNotFound
	}

	var code []byte
	code, err = a.GetCode(address)
	if err != nil {
		return nil, err
	}

	var publicKeys []flow.AccountPublicKey
	publicKeys, err = a.GetPublicKeys(address)
	if err != nil {
		return nil, err
	}

	return &flow.Account{
		Address: address,
		Code:    code,
		Keys:    publicKeys,
	}, nil
}

func (a *Accounts) Exists(address flow.Address) (bool, error) {
	exists, err := a.ledger.Get(RegisterID(string(address.Bytes()), "", keyExists))
	if err != nil {
		return false, newLedgerGetError(keyExists, address, err)
	}

	if len(exists) != 0 {
		return true, nil
	}

	return false, nil
}

func (a *Accounts) Create(publicKeys []flow.AccountPublicKey) (flow.Address, error) {
	addressState, err := a.addresses.GetGeneratorState()
	if err != nil {
		return flow.EmptyAddress, err
	}

	// generate the new account address
	var newAddress flow.Address
	newAddress, err = addressState.NextAddress()
	if err != nil {
		return flow.EmptyAddress, err
	}

	// mark that this account exists
	a.ledger.Set(RegisterID(string(newAddress.Bytes()), "", keyExists), []byte{1})

	a.ledger.Set(RegisterID(string(newAddress.Bytes()), string(newAddress.Bytes()), keyCode), nil)

	err = a.SetPublicKeys(newAddress, publicKeys)
	if err != nil {
		return flow.EmptyAddress, err
	}

	// update the address state
	a.addresses.SetGeneratorState(addressState)

	return newAddress, nil
}

func (a *Accounts) GetPublicKeys(address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
	var countBytes []byte
	countBytes, err = a.ledger.Get(
		RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
	)
	if err != nil {
		return nil, newLedgerGetError(keyPublicKeyCount, address, err)
	}

	var count uint64

	if countBytes == nil {
		count = 0
	} else {
		countInt := new(big.Int).SetBytes(countBytes)
		if !countInt.IsUint64() {
			return nil, fmt.Errorf(
				"retrieved public key account count bytes (hex-encoded): %x do not represent valid uint64",
				countBytes,
			)
		}
		count = countInt.Uint64()
	}

	publicKeys = make([]flow.AccountPublicKey, count)

	for i := uint64(0); i < count; i++ {
		publicKey, err := a.ledger.Get(
			RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKey(i)),
		)
		if err != nil {
			return nil, newLedgerGetError(keyPublicKey(i), address, err)
		}

		if publicKey == nil {
			return nil, fmt.Errorf("failed to retrieve public key from account %s", address)
		}

		decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode public key: %w", err)
		}

		publicKeys[i] = decodedPublicKey
	}

	return publicKeys, nil
}

func (a *Accounts) SetPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {

	var existingCount uint64

	countBytes, err := a.ledger.Get(
		RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
	)
	if err != nil {
		return newLedgerGetError(keyPublicKeyCount, address, err)
	}

	if countBytes != nil {
		countInt := new(big.Int).SetBytes(countBytes)
		if !countInt.IsUint64() {
			return fmt.Errorf(
				"retrieved public key account bytes (hex): %x do not represent valid uint64",
				countBytes,
			)
		}
		existingCount = countInt.Uint64()
	} else {
		existingCount = 0
	}

	newCount := uint64(len(publicKeys)) // len returns int and this won't exceed uint64
	newKeyCount := new(big.Int).SetUint64(newCount)

	a.ledger.Set(
		RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount),
		newKeyCount.Bytes(),
	)

	for i, publicKey := range publicKeys {

		err = publicKey.Validate()
		if err != nil {
			return fmt.Errorf("invalid public key: %w", err)
		}

		publicKeyBytes, err := flow.EncodeAccountPublicKey(publicKey)
		if err != nil {
			return fmt.Errorf("failed to encode public key: %w", err)
		}

		// asserted length of publicKeys so i should always fit into uint64
		a.SetPublicKey(address, uint64(i), publicKeyBytes)
	}

	// delete leftover keys
	for i := newCount; i < existingCount; i++ {
		a.ledger.Delete(RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKey(i)))
	}

	return nil
}

func (a *Accounts) SetPublicKey(address flow.Address, keyID uint64, publicKey []byte) {
	a.ledger.Set(
		RegisterID(string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyID)),
		publicKey,
	)
}

func (a *Accounts) GetCode(address flow.Address) ([]byte, error) {
	key := RegisterID(
		string(address.Bytes()),
		string(address.Bytes()),
		keyCode,
	)

	code, err := a.ledger.Get(key)
	if err != nil {
		return nil, newLedgerGetError(keyCode, address, err)
	}

	return code, nil
}

func (a *Accounts) TouchCode(address flow.Address) {
	key := RegisterID(
		string(address.Bytes()),
		string(address.Bytes()),
		keyCode,
	)

	a.ledger.Touch(key)
}

func (a *Accounts) SetCode(address flow.Address, code []byte) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	key := RegisterID(string(address.Bytes()), string(address.Bytes()), keyCode)

	var prevCode []byte
	prevCode, err = a.ledger.Get(key)
	if err != nil {
		return fmt.Errorf("cannot retreive previous code: %w", err)
	}

	// skip updating if the new code equals the old
	if bytes.Equal(prevCode, code) {
		return nil
	}

	a.ledger.Set(key, code)

	return nil
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}
