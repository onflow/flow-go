package state

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"sort"

	"github.com/fxamacker/cbor/v2"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/ledger/common/utils"
	"github.com/onflow/flow-go/model/flow"
)

const (
	keyExists         = "exists"
	keyCode           = "code"
	keyContractNames  = "contract_names"
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
	contracts := make(map[string][]byte)
	contractNames, err := a.getContractNames(address)

	if err != nil {
		return nil, err
	}

	for _, name := range contractNames {
		contract, err := a.getContract(name, address)
		if err != nil {
			return nil, err
		}
		contracts[name] = contract
	}

	var publicKeys []flow.AccountPublicKey
	publicKeys, err = a.GetPublicKeys(address)
	if err != nil {
		return nil, err
	}

	return &flow.Account{
		Address:   address,
		Keys:      publicKeys,
		Contracts: contracts,
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

	storageUsedByStorageUsed := uint64(RegisterSize(newAddress, false, keyStorageUsed, make([]byte, uint64StorageSize)))
	err = a.setStorageUsed(newAddress, storageUsedByStorageUsed)
	if err != nil {
		return err
	}

	// mark that this account exists
	err = a.setValue(newAddress, false, keyExists, []byte{1})
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

	if len(publicKey) == 0 {
		return flow.AccountPublicKey{}, ErrAccountPublicKeyNotFound
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode public key: %w", err)
	}

	return decodedPublicKey, nil
}

func (a *Accounts) GetPublicKeyCount(address flow.Address) (uint64, error) {
	countBytes, err := a.getValue(address, true, keyPublicKeyCount)
	if err != nil {
		return 0, newLedgerGetError(keyPublicKeyCount, address, err)
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
	count, err := a.GetPublicKeyCount(address)
	if err != nil {
		return nil, fmt.Errorf("failed to get public key count of account: %w", err)
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
	count, err := a.GetPublicKeyCount(address)
	if err != nil {
		return err
	}

	_, err = a.SetPublicKey(address, count, publicKey)
	if err != nil {
		return err
	}

	return a.setPublicKeyCount(address, count+1)
}

func contractKey(contractName string) string {
	return fmt.Sprintf("%s.%s", keyCode, contractName)
}

func (a *Accounts) getContract(contractName string, address flow.Address) ([]byte, error) {

	contract, err := a.getValue(address,
		true,
		contractKey(contractName))
	if err != nil {
		return nil, newLedgerGetError(contractName, address, err)
	}

	return contract, nil
}

func (a *Accounts) setContract(contractName string, address flow.Address, contract []byte) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}

	var prevContract []byte
	prevContract, err = a.getValue(address, true, contractKey(contractName))
	if err != nil {
		return fmt.Errorf("cannot retreive previous contract: %w", err)
	}

	// skip updating if the new contract equals the old
	if bytes.Equal(prevContract, contract) {
		return nil
	}

	err = a.setValue(address, true, contractKey(contractName), contract)
	if err != nil {
		return err
	}

	return nil
}

func newLedgerGetError(key string, address flow.Address, err error) error {
	return fmt.Errorf("failed to read key %s on account %s: %w", key, address, err)
}

func (a *Accounts) setContractNames(contractNames contractNames, address flow.Address) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("account with address %s does not exist", address)
	}
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err = cborEncoder.Encode(contractNames)
	if err != nil {
		return fmt.Errorf("cannot encode contract names")
	}
	newContractNames := buf.Bytes()

	var prevContractNames []byte
	prevContractNames, err = a.getValue(address, true, keyContractNames)
	if err != nil {
		return fmt.Errorf("cannot retrieve current contract names: %w", err)
	}

	// skip updating if the new contract names equal the old
	if bytes.Equal(prevContractNames, newContractNames) {
		return nil
	}

	return a.setValue(address, true, keyContractNames, newContractNames)
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

	storageUsed, _, err := utils.ReadUint64(storageUsedRegister)
	if err != nil {
		return 0, err
	}
	return storageUsed, nil
}

func (a *Accounts) setStorageUsed(address flow.Address, used uint64) error {
	usedBinary := utils.Uint64ToBinary(used)
	return a.setValue(address, false, keyStorageUsed, usedBinary)
}

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

	sizeChange := int64(RegisterSize(address, isController, key, value) - RegisterSize(address, isController, key, oldValue))
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

func RegisterSize(address flow.Address, isController bool, key string, value flow.RegisterValue) int {
	if len(value) == 0 {
		// registers with empty value won't (or don't) exist when stored
		return 0
	}

	var registerID flow.RegisterID
	if isController {
		registerID = flow.NewRegisterID(string(address.Bytes()), string(address.Bytes()), key)
	} else {
		registerID = flow.NewRegisterID(string(address.Bytes()), "", key)
	}
	registerKey := state.RegisterIDToKey(registerID)
	return registerKey.Size() + len(value)
}

func (a *Accounts) touch(address flow.Address, isController bool, key string) {
	if isController {
		a.ledger.Touch(string(address.Bytes()), string(address.Bytes()), key)
	} else {
		a.ledger.Touch(string(address.Bytes()), "", key)
	}
}

func (a *Accounts) TouchContract(contractName string, address flow.Address) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		panic(err)
	}
	if contractNames.Has(contractName) {
		a.touch(address,
			true,
			contractKey(contractName))
	}
}

// GetContractNames gets a sorted list of names of contracts deployed on an address
func (a *Accounts) GetContractNames(address flow.Address) ([]string, error) {
	return a.getContractNames(address)
}

func (a *Accounts) getContractNames(address flow.Address) (contractNames, error) {
	encContractNames, err := a.getValue(address, true, keyContractNames)
	if err != nil {
		return nil, fmt.Errorf("cannot get deployed contract names: %w", err)
	}
	identifiers := make([]string, 0)
	if len(encContractNames) > 0 {
		buf := bytes.NewReader(encContractNames)
		cborDecoder := cbor.NewDecoder(buf)
		err = cborDecoder.Decode(&identifiers)
		if err != nil {
			return nil, fmt.Errorf("cannot decode deployed contract names %x: %w", encContractNames, err)
		}
	}
	return identifiers, nil
}

func (a *Accounts) GetContract(contractName string, address flow.Address) ([]byte, error) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return nil, err
	}
	if !contractNames.Has(contractName) {
		return nil, nil
	}
	return a.getContract(contractName, address)
}

func (a *Accounts) SetContract(contractName string, address flow.Address, contract []byte) error {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return err
	}
	err = a.setContract(contractName, address, contract)
	if err != nil {
		return err
	}
	contractNames.add(contractName)
	return a.setContractNames(contractNames, address)
}

func (a *Accounts) DeleteContract(contractName string, address flow.Address) error {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return err
	}
	if !contractNames.Has(contractName) {
		return nil
	}
	err = a.setContract(contractName, address, nil)
	if err != nil {
		return err
	}
	contractNames.remove(contractName)
	return a.setContractNames(contractNames, address)
}

// contractNames container for a list of contract names. Should always be sorted.
// To ensure this, don't sort while reading it from storage, but sort it while adding/removing elements
type contractNames []string

func (l contractNames) Has(contractNames string) bool {
	i := sort.SearchStrings(l, contractNames)
	return i != len(l) && l[i] == contractNames
}
func (l *contractNames) add(contractNames string) {
	i := sort.SearchStrings(*l, contractNames)
	if i != len(*l) && (*l)[i] == contractNames {
		// list already contains element
		return
	}
	*l = append(*l, "")
	copy((*l)[i+1:], (*l)[i:])
	(*l)[i] = contractNames
}
func (l *contractNames) remove(contractName string) {
	i := sort.SearchStrings(*l, contractName)
	if i == len(*l) || (*l)[i] != contractName {
		// list doesnt contain the element
		return
	}
	*l = append((*l)[:i], (*l)[i+1:]...)
}
