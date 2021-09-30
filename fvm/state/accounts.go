package state

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"sort"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

const (
	KeyExists             = "exists"
	KeyCode               = "code"
	KeyContractNames      = "contract_names"
	KeyPublicKeyCount     = "public_key_count"
	KeyStorageUsed        = "storage_used"
	KeyAccountFrozen      = "frozen"
	KeyStorageIndex       = "storage_index"
	uint64StorageSize     = 8
	AccountFrozenValue    = 1
	AccountNotFrozenValue = 0
)

func keyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}

type Accounts interface {
	Exists(address flow.Address) (bool, error)
	GetPublicKeyCount(address flow.Address) (uint64, error)
	AppendPublicKey(address flow.Address, key flow.AccountPublicKey) error
	GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error)
	SetPublicKey(address flow.Address, keyIndex uint64, publicKey flow.AccountPublicKey) ([]byte, error)
	GetContractNames(address flow.Address) ([]string, error)
	GetContract(contractName string, address flow.Address) ([]byte, error)
	SetContract(contractName string, address flow.Address, contract []byte) error
	DeleteContract(contractName string, address flow.Address) error
	Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error
	GetValue(address flow.Address, key string) (flow.RegisterValue, error)
	CheckAccountNotFrozen(address flow.Address) error
	GetStorageUsed(address flow.Address) (uint64, error)
	SetValue(address flow.Address, key string, value []byte) error
	AllocateStorageIndex(address flow.Address) (atree.StorageIndex, error)
	SetAccountFrozen(address flow.Address, frozen bool) error
}

var _ Accounts = &StatefulAccounts{}

type StatefulAccounts struct {
	stateHolder *StateHolder
}

func NewAccounts(stateHolder *StateHolder) *StatefulAccounts {
	return &StatefulAccounts{
		stateHolder: stateHolder,
	}
}

func (a *StatefulAccounts) AllocateStorageIndex(address flow.Address) (atree.StorageIndex, error) {
	indexBytes, err := a.getValue(address, false, KeyStorageIndex)
	if err != nil {
		return atree.StorageIndex{}, err
	}

	if len(indexBytes) == 0 {
		// if not exist for the first time set it to zero and return
		indexBytes = []byte{0, 0, 0, 0, 0, 0, 0, 1}
	} else if len(indexBytes) != uint64StorageSize {
		return atree.StorageIndex{}, fmt.Errorf("invalid storage index byte size (%d != 8)", len(indexBytes))
	}

	var index atree.StorageIndex
	copy(index[:], indexBytes[:8])
	newIndexBytes := index.Next()

	err = a.setValue(address, false, KeyStorageIndex, newIndexBytes[:])
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("failed to store the key storage index: %w", err)
	}
	return index, nil
}

func (a *StatefulAccounts) Get(address flow.Address) (*flow.Account, error) {
	var ok bool
	var err error

	ok, err = a.Exists(address)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.NewAccountNotFoundError(address)
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

func (a *StatefulAccounts) Exists(address flow.Address) (bool, error) {
	exists, err := a.getValue(address, false, KeyExists)
	if err != nil {
		return false, err
	}

	if len(exists) != 0 {
		return true, nil
	}

	return false, nil
}

// Create account sets all required registers on an address.
func (a *StatefulAccounts) Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error {
	exists, err := a.Exists(newAddress)
	if err != nil {
		return err
	}
	if exists {
		return errors.NewAccountAlreadyExistsError(newAddress)
	}

	storageUsedByStorageUsed := uint64(RegisterSize(newAddress, false, KeyStorageUsed, make([]byte, uint64StorageSize)))
	err = a.setStorageUsed(newAddress, storageUsedByStorageUsed)
	if err != nil {
		return err
	}

	// mark that this account exists
	err = a.setValue(newAddress, false, KeyExists, []byte{1})
	if err != nil {
		return err
	}
	return a.SetAllPublicKeys(newAddress, publicKeys)
}

func (a *StatefulAccounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := a.getValue(address, true, keyPublicKey(keyIndex))
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	if len(publicKey) == 0 {
		return flow.AccountPublicKey{}, errors.NewAccountPublicKeyNotFoundError(address, keyIndex)
	}

	decodedPublicKey, err := flow.DecodeAccountPublicKey(publicKey, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to decode public key: %w", err)
	}

	return decodedPublicKey, nil
}

func (a *StatefulAccounts) GetPublicKeyCount(address flow.Address) (uint64, error) {
	countBytes, err := a.getValue(address, true, KeyPublicKeyCount)
	if err != nil {
		return 0, err
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

func (a *StatefulAccounts) setPublicKeyCount(address flow.Address, count uint64) error {
	newCount := new(big.Int).SetUint64(count)

	return a.setValue(address, true, KeyPublicKeyCount, newCount.Bytes())
}

func (a *StatefulAccounts) GetPublicKeys(address flow.Address) (publicKeys []flow.AccountPublicKey, err error) {
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

func (a *StatefulAccounts) SetPublicKey(
	address flow.Address,
	keyIndex uint64,
	publicKey flow.AccountPublicKey,
) (encodedPublicKey []byte, err error) {
	err = publicKey.Validate()
	if err != nil {
		encoded, _ := publicKey.MarshalJSON()
		return nil, errors.NewValueErrorf(string(encoded), "invalid public key value: %w", err)
	}

	encodedPublicKey, err = flow.EncodeAccountPublicKey(publicKey)
	if err != nil {
		encoded, _ := publicKey.MarshalJSON()
		return nil, errors.NewValueErrorf(string(encoded), "invalid public key value: %w", err)
	}

	err = a.setValue(address, true, keyPublicKey(keyIndex), encodedPublicKey)

	return encodedPublicKey, err
}

func (a *StatefulAccounts) SetAllPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {
	for i, publicKey := range publicKeys {
		_, err := a.SetPublicKey(address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

	count := uint64(len(publicKeys)) // len returns int and this will not exceed uint64

	return a.setPublicKeyCount(address, count)
}

func (a *StatefulAccounts) AppendPublicKey(address flow.Address, publicKey flow.AccountPublicKey) error {

	if !IsValidAccountKeyHashAlgo(publicKey.HashAlgo) {
		return errors.NewValueErrorf(publicKey.HashAlgo.String(), "hashing algorithm type not found")
	}

	if !IsValidAccountKeySignAlgo(publicKey.SignAlgo) {
		return errors.NewValueErrorf(publicKey.SignAlgo.String(), "signature algorithm type not found")
	}

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

func IsValidAccountKeySignAlgo(algo crypto.SigningAlgorithm) bool {
	switch algo {
	case crypto.ECDSAP256, crypto.ECDSASecp256k1:
		return true
	default:
		return false
	}
}

func IsValidAccountKeyHashAlgo(algo hash.HashingAlgorithm) bool {
	switch algo {
	case hash.SHA2_256, hash.SHA3_256:
		return true
	default:
		return false
	}
}

func ContractKey(contractName string) string {
	return fmt.Sprintf("%s.%s", KeyCode, contractName)
}

func (a *StatefulAccounts) getContract(contractName string, address flow.Address) ([]byte, error) {

	contract, err := a.getValue(address,
		true,
		ContractKey(contractName))
	if err != nil {
		return nil, err
	}

	return contract, nil
}

func (a *StatefulAccounts) setContract(contractName string, address flow.Address, contract []byte) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return errors.NewAccountNotFoundError(address)
	}

	var prevContract []byte
	prevContract, err = a.getValue(address, true, ContractKey(contractName))
	if err != nil {
		return errors.NewContractNotFoundError(address, contractName)
	}

	// skip updating if the new contract equals the old
	if bytes.Equal(prevContract, contract) {
		return nil
	}

	err = a.setValue(address, true, ContractKey(contractName), contract)
	if err != nil {
		return err
	}

	return nil
}

func (a *StatefulAccounts) setContractNames(contractNames contractNames, address flow.Address) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return errors.NewAccountNotFoundError(address)
	}
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err = cborEncoder.Encode(contractNames)
	if err != nil {
		msg := fmt.Sprintf("cannot encode contract names: %s", contractNames)
		return errors.NewEncodingFailuref(msg, err)
	}
	newContractNames := buf.Bytes()

	var prevContractNames []byte
	prevContractNames, err = a.getValue(address, true, KeyContractNames)
	if err != nil {
		return fmt.Errorf("cannot retrieve current contract names: %w", err)
	}

	// skip updating if the new contract names equal the old
	if bytes.Equal(prevContractNames, newContractNames) {
		return nil
	}

	return a.setValue(address, true, KeyContractNames, newContractNames)
}

// GetStorageUsed returns the amount of storage used in bytes by this account
func (a *StatefulAccounts) GetStorageUsed(address flow.Address) (uint64, error) {
	storageUsedRegister, err := a.getValue(address, false, KeyStorageUsed)
	if err != nil {
		return 0, err
	}

	if len(storageUsedRegister) != uint64StorageSize {
		return 0, fmt.Errorf("account %s storage used is not initialized or not initialized correctly", address.Hex())
	}

	storageUsed, _, err := readUint64(storageUsedRegister)
	if err != nil {
		return 0, err
	}
	return storageUsed, nil
}

func (a *StatefulAccounts) setStorageUsed(address flow.Address, used uint64) error {
	usedBinary := uint64ToBinary(used)
	return a.setValue(address, false, KeyStorageUsed, usedBinary)
}

func (a *StatefulAccounts) GetValue(address flow.Address, key string) (flow.RegisterValue, error) {
	return a.getValue(address, false, key)
}

func (a *StatefulAccounts) getValue(address flow.Address, isController bool, key string) (flow.RegisterValue, error) {
	if isController {
		return a.stateHolder.State().Get(string(address.Bytes()), string(address.Bytes()), key)
	}
	return a.stateHolder.State().Get(string(address.Bytes()), "", key)
}

// SetValue sets a value in address' storage
func (a *StatefulAccounts) SetValue(address flow.Address, key string, value flow.RegisterValue) error {
	return a.setValue(address, false, key, value)
}

func (a *StatefulAccounts) setValue(address flow.Address, isController bool, key string, value flow.RegisterValue) error {
	err := a.updateRegisterSizeChange(address, isController, key, value)
	if err != nil {
		return fmt.Errorf("failed to update storage used by key %s on account %s: %w", key, address, err)
	}

	if isController {
		return a.stateHolder.State().Set(string(address.Bytes()), string(address.Bytes()), key, value)
	}
	return a.stateHolder.State().Set(string(address.Bytes()), "", key, value)
}

func (a *StatefulAccounts) updateRegisterSizeChange(address flow.Address, isController bool, key string, value flow.RegisterValue) error {
	if key == KeyStorageUsed {
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

	return getRegisterIDSize(registerID) + len(value)
}

// TODO replace with touch
// TODO handle errors
func (a *StatefulAccounts) touch(address flow.Address, isController bool, key string) {
	if isController {
		_, _ = a.stateHolder.State().Get(string(address.Bytes()), string(address.Bytes()), key)
		return
	}
	_, _ = a.stateHolder.State().Get(string(address.Bytes()), "", key)
}

func (a *StatefulAccounts) TouchContract(contractName string, address flow.Address) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		panic(err)
	}
	if contractNames.Has(contractName) {
		a.touch(address,
			true,
			ContractKey(contractName))
	}
}

// GetContractNames gets a sorted list of names of contracts deployed on an address
func (a *StatefulAccounts) GetContractNames(address flow.Address) ([]string, error) {
	return a.getContractNames(address)
}

func (a *StatefulAccounts) getContractNames(address flow.Address) (contractNames, error) {
	// TODO return fatal error if can't fetch
	encContractNames, err := a.getValue(address, true, KeyContractNames)
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

func (a *StatefulAccounts) GetContract(contractName string, address flow.Address) ([]byte, error) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return nil, err
	}
	if !contractNames.Has(contractName) {
		return nil, nil
	}
	return a.getContract(contractName, address)
}

func (a *StatefulAccounts) SetContract(contractName string, address flow.Address, contract []byte) error {
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

func (a *StatefulAccounts) DeleteContract(contractName string, address flow.Address) error {
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

// This tries to compute the amount bytes that will be used by the ledger
// plus 2 on each part of the register id is due to header byte size needed
// for encoding and decoding
func getRegisterIDSize(inp flow.RegisterID) int {
	size := 0
	size += 2 + len(inp.Owner)
	size += 2 + len(inp.Controller)
	size += 2 + len(inp.Key)
	return size
}

// uint64ToBinary converst a uint64 to a byte slice (big endian)
func uint64ToBinary(integer uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, integer)
	return b
}

// readUint64 reads a uint64 from the input and returns the rest
func readUint64(input []byte) (value uint64, rest []byte, err error) {
	if len(input) < 8 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint64", len(input))
	}
	return binary.BigEndian.Uint64(input[:8]), input[8:], nil
}

func (a *StatefulAccounts) GetAccountFrozen(address flow.Address) (bool, error) {
	frozen, err := a.getValue(address, false, KeyAccountFrozen)
	if err != nil {
		return false, err
	}

	if len(frozen) == 0 {
		return false, err
	}

	return frozen[0] != AccountNotFrozenValue, nil
}

func (a *StatefulAccounts) SetAccountFrozen(address flow.Address, frozen bool) error {

	val := make([]byte, 1) //zero value for byte is 0
	if frozen {
		val[0] = AccountFrozenValue
	}

	return a.setValue(address, false, KeyAccountFrozen, val)
}

// handy function to error out if account is frozen
func (a *StatefulAccounts) CheckAccountNotFrozen(address flow.Address) error {
	frozen, err := a.GetAccountFrozen(address)
	if err != nil {
		return fmt.Errorf("cannot check account freeze status: %w", err)
	}
	if frozen {
		return errors.NewFrozenAccountError(address)
	}
	return nil
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
		// list does not contain the element
		return
	}
	*l = append((*l)[:i], (*l)[i+1:]...)
}
