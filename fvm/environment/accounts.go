package environment

import (
	"bytes"
	"fmt"
	"math"
	"sort"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

const (
	MaxPublicKeyCount = math.MaxUint64
)

type Accounts interface {
	Exists(address flow.Address) (bool, error)
	Get(address flow.Address) (*flow.Account, error)
	GetPublicKeyCount(address flow.Address) (uint64, error)
	AppendPublicKey(address flow.Address, key flow.AccountPublicKey) error
	GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error)
	SetPublicKey(address flow.Address, keyIndex uint64, publicKey flow.AccountPublicKey) ([]byte, error)
	GetContractNames(address flow.Address) ([]string, error)
	GetContract(contractName string, address flow.Address) ([]byte, error)
	ContractExists(contractName string, address flow.Address) (bool, error)
	SetContract(contractName string, address flow.Address, contract []byte) error
	DeleteContract(contractName string, address flow.Address) error
	Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error
	GetValue(address flow.Address, key string) (flow.RegisterValue, error)
	CheckAccountNotFrozen(address flow.Address) error
	GetStorageUsed(address flow.Address) (uint64, error)
	SetValue(address flow.Address, key string, value flow.RegisterValue) error
	AllocateStorageIndex(address flow.Address) (atree.StorageIndex, error)
	SetAccountFrozen(address flow.Address, frozen bool) error
}

var _ Accounts = &StatefulAccounts{}

type StatefulAccounts struct {
	txnState *state.TransactionState
}

func NewAccounts(txnState *state.TransactionState) *StatefulAccounts {
	return &StatefulAccounts{
		txnState: txnState,
	}
}

func (a *StatefulAccounts) AllocateStorageIndex(address flow.Address) (atree.StorageIndex, error) {
	// get status
	status, err := a.getAccountStatus(address)
	if err != nil {
		return atree.StorageIndex{}, err
	}

	// get and increment the index
	index := status.StorageIndex()
	newIndexBytes := index.Next()

	// store nil so that the setValue for new allocated slabs would be faster
	// and won't do ledger getValue for every new slabs (currently happening to compute storage size changes)
	// this way the getValue would load this value from deltas
	key := atree.SlabIndexToLedgerKey(index)
	a.txnState.RunWithAllLimitsDisabled(func() {
		err = a.txnState.Set(
			flow.RegisterID{
				Owner: string(address.Bytes()),
				Key:   string(key),
			},
			[]byte{})
	})
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("failed to allocate an storage index: %w", err)
	}

	// update the storageIndex bytes
	status.SetStorageIndex(newIndexBytes)
	err = a.setAccountStatus(address, status)
	if err != nil {
		return atree.StorageIndex{}, fmt.Errorf("failed to allocate an storage index: %w", err)
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
	accStatusBytes, err := a.GetValue(address, state.AccountStatusKey)
	if err != nil {
		return false, err
	}

	// account doesn't exist if account status doesn't exist
	if len(accStatusBytes) == 0 {
		return false, nil
	}

	// check if we can construct account status from the value of this register
	_, err = AccountStatusFromBytes(accStatusBytes)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create account sets all required registers on an address.
func (a *StatefulAccounts) Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error {
	exists, err := a.Exists(newAddress)
	if err != nil {
		return fmt.Errorf("failed to create a new account: %w", err)
	}
	if exists {
		return errors.NewAccountAlreadyExistsError(newAddress)
	}

	accountStatus := NewAccountStatus()
	storageUsedByTheStatusItself := uint64(RegisterSize(newAddress, state.AccountStatusKey, accountStatus.ToBytes()))
	err = a.setAccountStatusStorageUsed(newAddress, accountStatus, storageUsedByTheStatusItself)
	if err != nil {
		return fmt.Errorf("failed to create a new account: %w", err)
	}

	return a.SetAllPublicKeys(newAddress, publicKeys)
}

func (a *StatefulAccounts) GetPublicKey(address flow.Address, keyIndex uint64) (flow.AccountPublicKey, error) {
	publicKey, err := a.GetValue(address, KeyPublicKey(keyIndex))
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
	status, err := a.getAccountStatus(address)
	if err != nil {
		return 0, fmt.Errorf("failed to get public key count: %w", err)
	}
	return status.PublicKeyCount(), nil
}

func (a *StatefulAccounts) setPublicKeyCount(address flow.Address, count uint64) error {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return fmt.Errorf("failed to set public key count for account (%s): %w", address.String(), err)
	}

	status.SetPublicKeyCount(count)

	err = a.setAccountStatus(address, status)
	if err != nil {
		return fmt.Errorf("failed to set public key count for account (%s): %w", address.String(), err)
	}
	return nil
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

	err = a.SetValue(address, KeyPublicKey(keyIndex), encodedPublicKey)

	return encodedPublicKey, err
}

func (a *StatefulAccounts) SetAllPublicKeys(address flow.Address, publicKeys []flow.AccountPublicKey) error {

	count := uint64(len(publicKeys)) // len returns int and this will not exceed uint64

	if count >= MaxPublicKeyCount {
		return errors.NewAccountPublicKeyLimitError(address, count, MaxPublicKeyCount)
	}

	for i, publicKey := range publicKeys {
		_, err := a.SetPublicKey(address, uint64(i), publicKey)
		if err != nil {
			return err
		}
	}

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

	if count >= MaxPublicKeyCount {
		return errors.NewAccountPublicKeyLimitError(address, count+1, MaxPublicKeyCount)
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
	return state.CodeKeyPrefix + contractName
}

func (a *StatefulAccounts) getContract(contractName string, address flow.Address) ([]byte, error) {

	contract, err := a.GetValue(address, ContractKey(contractName))
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
	prevContract, err = a.GetValue(address, ContractKey(contractName))
	if err != nil {
		return errors.NewContractNotFoundError(address, contractName)
	}

	// skip updating if the new contract equals the old
	if bytes.Equal(prevContract, contract) {
		return nil
	}

	err = a.SetValue(address, ContractKey(contractName), contract)
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
		return errors.NewEncodingFailuref(
			err,
			"cannot encode contract names: %s",
			contractNames)
	}
	newContractNames := buf.Bytes()

	var prevContractNames []byte
	prevContractNames, err = a.GetValue(address, state.ContractNamesKey)
	if err != nil {
		return fmt.Errorf("cannot retrieve current contract names: %w", err)
	}

	// skip updating if the new contract names equal the old
	if bytes.Equal(prevContractNames, newContractNames) {
		return nil
	}

	return a.SetValue(address, state.ContractNamesKey, newContractNames)
}

// GetStorageUsed returns the amount of storage used in bytes by this account
func (a *StatefulAccounts) GetStorageUsed(address flow.Address) (uint64, error) {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage used: %w", err)
	}
	return status.StorageUsed(), nil
}

func (a *StatefulAccounts) setStorageUsed(address flow.Address, used uint64) error {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return fmt.Errorf("failed to set storage used: %w", err)
	}

	return a.setAccountStatusStorageUsed(address, status, used)
}

func (a *StatefulAccounts) setAccountStatusStorageUsed(address flow.Address, status *AccountStatus, newUsed uint64) error {
	status.SetStorageUsed(newUsed)

	err := a.setAccountStatus(address, status)
	if err != nil {
		return fmt.Errorf("failed to set storage used: %w", err)
	}
	return nil
}

// TODO(patrick): use RegisterID as input key
func (a *StatefulAccounts) GetValue(address flow.Address, key string) (flow.RegisterValue, error) {
	return a.txnState.Get(flow.RegisterID{
		Owner: string(address.Bytes()),
		Key:   key,
	})
}

// TODO(patrick): use RegisterID as input key
// SetValue sets a value in address' storage
func (a *StatefulAccounts) SetValue(address flow.Address, key string, value flow.RegisterValue) error {
	err := a.updateRegisterSizeChange(address, key, value)
	if err != nil {
		return fmt.Errorf("failed to update storage used by key %s on account %s: %w", state.PrintableKey(key), address, err)
	}
	return a.txnState.Set(flow.RegisterID{
		Owner: string(address.Bytes()),
		Key:   key},
		value)
}

func (a *StatefulAccounts) updateRegisterSizeChange(address flow.Address, key string, value flow.RegisterValue) error {
	if key == state.AccountStatusKey {
		// size of this register is always fixed size
		// don't double check this to save time and prevent recursion
		return nil
	}
	oldValue, err := a.GetValue(address, key)
	if err != nil {
		return err
	}

	sizeChange := int64(RegisterSize(address, key, value) - RegisterSize(address, key, oldValue))
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
			return fmt.Errorf("storage used by key %s on account %s would be negative", state.PrintableKey(key), address.Hex())
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

func RegisterSize(address flow.Address, key string, value flow.RegisterValue) int {
	if len(value) == 0 {
		// registers with empty value won't (or don't) exist when stored
		return 0
	}
	size := 0
	// additional 2 is for len prefixes when encoding is happening
	// we might get rid of these 2s in the future
	size += 2 + len(string(address.Bytes()))
	size += 2 + len(key)
	size += len(value)
	return size
}

// GetContractNames gets a sorted list of names of contracts deployed on an address
func (a *StatefulAccounts) GetContractNames(address flow.Address) ([]string, error) {
	return a.getContractNames(address)
}

func (a *StatefulAccounts) getContractNames(address flow.Address) (contractNames, error) {
	// TODO return fatal error if can't fetch
	encContractNames, err := a.GetValue(address, state.ContractNamesKey)
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

func (a *StatefulAccounts) ContractExists(contractName string, address flow.Address) (bool, error) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return false, err
	}
	return contractNames.Has(contractName), nil
}

func (a *StatefulAccounts) GetContract(contractName string, address flow.Address) ([]byte, error) {
	// we optimized the happy case here, we look up for the content of the contract
	// and if its not there we check if contract exists or if this is another problem.
	code, err := a.getContract(contractName, address)
	if err != nil || len(code) == 0 {
		exists, err := a.ContractExists(contractName, address)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, nil
		}
	}
	return code, err
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

func (a *StatefulAccounts) getAccountStatus(address flow.Address) (*AccountStatus, error) {
	statusBytes, err := a.GetValue(address, state.AccountStatusKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load account status for the account (%s): %w", address.String(), err)
	}
	if len(statusBytes) == 0 {
		return nil, errors.NewAccountNotFoundError(address)
	}
	return AccountStatusFromBytes(statusBytes)
}

func (a *StatefulAccounts) setAccountStatus(address flow.Address, status *AccountStatus) error {
	err := a.SetValue(address, state.AccountStatusKey, status.ToBytes())
	if err != nil {
		return fmt.Errorf("failed to store the account status for account (%s): %w", address.String(), err)
	}
	return nil
}

func (a *StatefulAccounts) GetAccountFrozen(address flow.Address) (bool, error) {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return false, err
	}
	return status.IsAccountFrozen(), nil
}

func (a *StatefulAccounts) SetAccountFrozen(address flow.Address, frozen bool) error {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return err
	}
	status.SetFrozenFlag(frozen)
	return a.setAccountStatus(address, status)
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

func KeyPublicKey(index uint64) string {
	return fmt.Sprintf("public_key_%d", index)
}
