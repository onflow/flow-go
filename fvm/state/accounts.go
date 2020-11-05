package state

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/onflow/flow-go/model/flow"
)

const (
	keyExists         = "exists"
	keyCode           = "code"
	keyPublicKeyCount = "public_key_count"
	keyStorageUsed    = "storage_used"
	uint64StorageSize = 8
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
	exists, err := a.getValue(address, false, keyExists)
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
		return fmt.Errorf("account with address %s already exists", newAddress.Hex())
	}

	err = a.setStorageUsed(newAddress, uint64StorageSize) //set storage used to the size of storage used
	if err != nil {
		return err
	}

	// mark that this account exists
	err = a.setValue(newAddress, false, keyExists, []byte{1})
	if err != nil {
		return err
	}

	err = a.setValue(newAddress, true, keyCode, nil)
	if err != nil {
		return err
	}

	return a.SetAllPublicKeys(newAddress, publicKeys)
}

func (a *Accounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := a.getValue(address, true, keyPublicKey(keyIndex))
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

func (a *Accounts) getPublicKeyCount(address flow.Address) (uint64, error) {
	countBytes, err := a.getValue(address, true, keyPublicKeyCount)
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

func (a *Accounts) setPublicKeyCount(address flow.Address, count uint64) error {
	newCount := new(big.Int).SetUint64(count)

	return a.setValue(address, true, keyPublicKeyCount, newCount.Bytes())
}

func (a *Accounts) GetPublicKeys(address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
	var countBytes []byte
	countBytes, err = a.getValue(address, true, keyPublicKeyCount)
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

	err = a.setValue(address, true, keyPublicKey(keyIndex), encodedPublicKey)

	return encodedPublicKey, err
}

func (a *Accounts) SetAllPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {
	for i, publicKey := range publicKeys {
		_, err := a.SetPublicKey(address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

	count := uint64(len(publicKeys)) // len returns int and this will not exceed uint64

	return a.setPublicKeyCount(address, count)
}

func (a *Accounts) AppendPublicKey(address flow.Address, publicKey flow.AccountPublicKey) error {
	count, err := a.getPublicKeyCount(address)
	if err != nil {
		return err
	}

	_, err = a.SetPublicKey(address, count, publicKey)
	if err != nil {
		return err
	}

	return a.setPublicKeyCount(address, count+1)
}

func (a *Accounts) GetCode(address flow.Address) ([]byte, error) {

	code, err := a.getValue(address, true, keyCode)
	if err != nil {
		return nil, newLedgerGetError(keyCode, address, err)
	}

	return code, nil
}

func (a *Accounts) TouchCode(address flow.Address) {
	a.touch(address, true, keyCode)
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
	prevCode, err = a.getValue(address, true, keyCode)
	if err != nil {
		return fmt.Errorf("cannot retreive previous code: %w", err)
	}

	// skip updating if the new code equals the old
	if bytes.Equal(prevCode, code) {
		return nil
	}

	return a.setValue(address, true, keyCode, code)
}

// GetStorageUsed returns the amount of storage used in bytes by this account
func (a *Accounts) GetStorageUsed(address flow.Address) (uint64, error) {
	storageUsedRegister, err := a.getValue(address, false, keyStorageUsed)
	if err != nil {
		return 0, err
	}

	if len(storageUsedRegister) != uint64StorageSize {
		return 0, fmt.Errorf("account %s storage used is not initialized or not initialized correctly", address.Hex())
	}

	storageUsed := binary.LittleEndian.Uint64(storageUsedRegister)
	return storageUsed, nil
}

func (a *Accounts) setStorageUsed(address flow.Address, used uint64) error {
	buffer := make([]byte, uint64StorageSize)
	binary.LittleEndian.PutUint64(buffer, used)
	return a.setValue(address, false, keyStorageUsed, buffer)
}

// GetValue returns a value stored in address' storage
func (a *Accounts) GetValue(address flow.Address, key string) (flow.RegisterValue, error) {
	return a.getValue(address, false, key)
}

func (a *Accounts) getValue(address flow.Address, isController bool, key string) (flow.RegisterValue, error) {
	if isController {
		return a.ledger.Get(string(address.Bytes()), string(address.Bytes()), key)
	}
	return a.ledger.Get(string(address.Bytes()), "", key)
}

// SetValue sets a value in address' storage
func (a *Accounts) SetValue(address flow.Address, key string, value flow.RegisterValue) error {
	return a.setValue(address, false, key, value)
}

func (a *Accounts) setValue(address flow.Address, isController bool, key string, value flow.RegisterValue) error {
	err := a.updateRegisterSizeChange(address, isController, key, value)
	if err != nil {
		return fmt.Errorf("failed to update storage used by key %s on account %s: %w", key, address, err)
	}

	if isController {
		a.ledger.Set(string(address.Bytes()), string(address.Bytes()), key, value)
	} else {
		a.ledger.Set(string(address.Bytes()), "", key, value)
	}
	return nil
}

func (a *Accounts) updateRegisterSizeChange(address flow.Address, isController bool, key string, value flow.RegisterValue) error {
	if key == keyStorageUsed {
		// size of this register is always uint64StorageSize
		// don't double check this to save time and prevent recursion
		return nil
	}
	oldValue, err := a.getValue(address, isController, key)
	if err != nil {
		return err
	}

	sizeChange := int64(len(value) - len(oldValue))
	if sizeChange == 0 {
		// register size has not changed. Nothing to do
		return nil
	}

	oldSize, err := a.GetStorageUsed(address)
	if err != nil {
		return err
	}

	// two paths to avoid casting uint to int
	var newSize uint64
	if sizeChange < 0 {
		absChange := uint64(-sizeChange)
		if absChange > oldSize {
			// should never happen
			return fmt.Errorf("storage used by key %s on account %s would be negative", key, address.Hex())
		}
		newSize = oldSize - absChange
	} else {
		absChange := uint64(sizeChange)
		newSize = oldSize + absChange
	}

	// this puts us back in the setValue method.
	// The difference is that storage_used update exits early from this function so there isn't any recursion.
	return a.setStorageUsed(address, newSize)
}

func (a *Accounts) touch(address flow.Address, isController bool, key string) {
	if isController {
		a.ledger.Touch(string(address.Bytes()), string(address.Bytes()), key)
	} else {
		a.ledger.Touch(string(address.Bytes()), "", key)
	}
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}
