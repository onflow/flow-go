package state

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/onflow/flow-go/model/flow"
)

const (
	keyExists         = "exists"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
)

var (
	ErrAccountNotFound          = errors.New("account not found")
	ErrAccountPublicKeyNotFound = errors.New("account public key not found")
)

func keyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}

type Accounts struct {
	ledger Ledger
}

func NewAccounts(ledger Ledger) *Accounts {
	return &Accounts{
		ledger: ledger,
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
	exists, err := a.ledger.Get(string(address.Bytes()), "", keyExists)
	if err != nil {
		return false, newLedgerGetError(keyExists, address, err)
	}

	if len(exists) != 0 {
		return true, nil
	}

	return false, nil
}

// Create account sets all required registers on an address.
func (a *Accounts) Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error {
	exists, err := a.Exists(newAddress)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("account already created on %s", newAddress.Hex())
	}

	// mark that this account exists
	a.ledger.Set(string(newAddress.Bytes()), "", keyExists, []byte{1})

	a.ledger.Set(string(newAddress.Bytes()), string(newAddress.Bytes()), keyCode, nil)

	return a.SetAllPublicKeys(newAddress, publicKeys)
}

func (a *Accounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex),
	)
	if err != nil {
		return flow.AccountPublicKey{}, newLedgerGetError(keyPublicKey(keyIndex), address, err)
	}

	if publicKey == nil {
		return flow.AccountPublicKey{}, ErrAccountPublicKeyNotFound
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode public key: %w", err)
	}

	return decodedPublicKey, nil
}

func (a *Accounts) GetPublicKeyCount(address flow.Address) (uint64, error) {
	countBytes, err := a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
	)
	if err != nil {
		return 0, newLedgerGetError(keyPublicKeyCount, address, err)
	}

	if countBytes == nil {
		return 0, nil
	}

	countInt := new(big.Int).SetBytes(countBytes)
	if !countInt.IsUint64() {
		return 0, fmt.Errorf(
			"retrieved public key account count bytes (hex-encoded): %x does not represent valid uint64",
			countBytes,
		)
	}

	return countInt.Uint64(), nil
}

func (a *Accounts) SetPublicKeyCount(address flow.Address, count uint64) {
	newCount := new(big.Int).SetUint64(count)

	a.ledger.Set(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
		newCount.Bytes(),
	)
}

func (a *Accounts) GetPublicKeys(address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
	var countBytes []byte
	countBytes, err = a.ledger.Get(
		string(address.Bytes()), string(address.Bytes()), keyPublicKeyCount,
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
		publicKey, err := a.GetPublicKey(address, i)
		if err != nil {
			return nil, err
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func (a *Accounts) SetPublicKey(
	address flow.Address,
	keyIndex uint64,
	publicKey flow.AccountPublicKey,
) (encodedPublicKey []byte, err error) {
	err = publicKey.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}

	encodedPublicKey, err = flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}

	a.ledger.Set(
		string(address.Bytes()), string(address.Bytes()), keyPublicKey(keyIndex),
		encodedPublicKey,
	)

	return encodedPublicKey, nil
}

func (a *Accounts) SetAllPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {

	for i, publicKey := range publicKeys {
		_, err := a.SetPublicKey(address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

	count := uint64(len(publicKeys)) // len returns int and this will not exceed uint64

	a.SetPublicKeyCount(address, count)

	return nil
}

func (a *Accounts) AppendPublicKey(address flow.Address, publicKey flow.AccountPublicKey) error {
	count, err := a.GetPublicKeyCount(address)
	if err != nil {
		return err
	}

	_, err = a.SetPublicKey(address, count, publicKey)
	if err != nil {
		return err
	}

	a.SetPublicKeyCount(address, count+1)

	return nil
}

func (a *Accounts) GetCode(address flow.Address) ([]byte, error) {

	code, err := a.ledger.Get(string(address.Bytes()),
		string(address.Bytes()),
		keyCode)
	if err != nil {
		return nil, newLedgerGetError(keyCode, address, err)
	}

	return code, nil
}

func (a *Accounts) TouchCode(address flow.Address) {

	a.ledger.Touch(string(address.Bytes()),
		string(address.Bytes()),
		keyCode)
}

func (a *Accounts) SetCode(address flow.Address, code []byte) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	var prevCode []byte
	prevCode, err = a.ledger.Get(string(address.Bytes()), string(address.Bytes()), keyCode)
	if err != nil {
		return fmt.Errorf("cannot retreive previous code: %w", err)
	}

	// skip updating if the new code equals the old
	if bytes.Equal(prevCode, code) {
		return nil
	}

	a.ledger.Set(string(address.Bytes()), string(address.Bytes()), keyCode, code)

	return nil
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}
