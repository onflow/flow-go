package environment

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/onflow/atree"
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	accountkeymetadata "github.com/onflow/flow-go/fvm/environment/account-key-metadata"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/model/flow"
)

const (
	MaxPublicKeyCount = math.MaxUint32
)

type Accounts interface {
	Exists(address flow.Address) (bool, error)
	Get(address flow.Address) (*flow.Account, error)
	GetAccountPublicKeyCount(address flow.Address) (uint32, error)
	AppendAccountPublicKey(address flow.Address, key flow.AccountPublicKey) error
	GetRuntimeAccountPublicKey(address flow.Address, keyIndex uint32) (flow.RuntimeAccountPublicKey, error)
	GetAccountPublicKey(address flow.Address, keyIndex uint32) (flow.AccountPublicKey, error)
	GetAccountPublicKeys(address flow.Address) ([]flow.AccountPublicKey, error)
	RevokeAccountPublicKey(address flow.Address, keyIndex uint32) error
	GetAccountPublicKeyRevokedStatus(address flow.Address, keyIndex uint32) (bool, error)
	GetAccountPublicKeySequenceNumber(address flow.Address, keyIndex uint32) (uint64, error)
	// IncrementAccountPublicKeySequenceNumber increments the sequence number for the account's public key
	// at the given key index.  This update does not affect the account status, enabling concurrent execution
	// of transactions that do not modify any data related to account status.
	// Note: No additional storage is consumed, as the storage for the sequence number register
	// was allocated when account public key was initially added to the account.
	IncrementAccountPublicKeySequenceNumber(address flow.Address, keyIndex uint32) error
	GetContractNames(address flow.Address) ([]string, error)
	GetContract(contractName string, address flow.Address) ([]byte, error)
	ContractExists(contractName string, address flow.Address) (bool, error)
	SetContract(contractName string, address flow.Address, contract []byte) error
	DeleteContract(contractName string, address flow.Address) error
	Create(publicKeys []flow.AccountPublicKey, newAddress flow.Address) error
	GetValue(id flow.RegisterID) (flow.RegisterValue, error)
	GetStorageUsed(address flow.Address) (uint64, error)
	SetValue(id flow.RegisterID, value flow.RegisterValue) error
	AllocateSlabIndex(address flow.Address) (atree.SlabIndex, error)
	GenerateAccountLocalID(address flow.Address) (uint64, error)
}

var _ Accounts = &StatefulAccounts{}

type StatefulAccounts struct {
	txnState state.NestedTransactionPreparer
}

func NewAccounts(txnState state.NestedTransactionPreparer) *StatefulAccounts {
	return &StatefulAccounts{
		txnState: txnState,
	}
}

func (a *StatefulAccounts) AllocateSlabIndex(
	address flow.Address,
) (
	atree.SlabIndex,
	error,
) {
	// get status
	status, err := a.getAccountStatus(address)
	if err != nil {
		return atree.SlabIndex{}, err
	}

	// get and increment the index
	index := status.SlabIndex()
	newIndexBytes := index.Next()

	// store nil so that the setValue for new allocated slabs would be faster
	// and won't do ledger getValue for every new slabs (currently happening to
	// compute storage size changes)
	// this way the getValue would load this value from deltas
	key := atree.SlabIndexToLedgerKey(index)
	a.txnState.RunWithMeteringDisabled(func() {
		err = a.txnState.Set(
			flow.NewRegisterID(address, string(key)),
			[]byte{})
	})
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf(
			"failed to allocate an storage index: %w",
			err)
	}

	// update the storageIndex bytes
	status.SetStorageIndex(newIndexBytes)
	err = a.setAccountStatus(address, status)
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf(
			"failed to allocate an storage index: %w",
			err)
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
	publicKeys, err = a.GetAccountPublicKeys(address)
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
	accStatusBytes, err := a.GetValue(flow.AccountStatusRegisterID(address))
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
func (a *StatefulAccounts) Create(
	publicKeys []flow.AccountPublicKey,
	newAddress flow.Address,
) error {
	exists, err := a.Exists(newAddress)
	if err != nil {
		return fmt.Errorf("failed to create a new account: %w", err)
	}
	if exists {
		return errors.NewAccountAlreadyExistsError(newAddress)
	}

	publicKeyCount := uint32(len(publicKeys))

	if publicKeyCount >= MaxPublicKeyCount {
		return errors.NewAccountPublicKeyLimitError(
			newAddress,
			publicKeyCount,
			MaxPublicKeyCount)
	}

	accountStatus := NewAccountStatus()
	storageUsedByTheStatusItself := uint64(RegisterSize(
		flow.AccountStatusRegisterID(newAddress),
		accountStatus.ToBytes()))
	err = a.setAccountStatusStorageUsed(
		newAddress,
		accountStatus,
		storageUsedByTheStatusItself)
	if err != nil {
		return fmt.Errorf("failed to create a new account: %w", err)
	}

	for i, publicKey := range publicKeys {
		err := a.appendPublicKey(newAddress, publicKey, uint32(i))
		if err != nil {
			return err
		}
	}

	// NOTE: do not include pre-allocated sequence number register size in storage used for account public key 0.

	if publicKeyCount <= 1 {
		return nil
	}

	// Adjust storage used for pre-allocated sequence number registers, starting from account public key 1.
	sequenceNumberPayloadSize := uint64(0)
	for i := uint32(1); i < publicKeyCount; i++ {
		sequenceNumberPayloadSize += PredefinedSequenceNumberPayloadSize(newAddress, i)
	}

	status, err := a.getAccountStatus(newAddress)
	if err != nil {
		return err
	}

	storageUsed := status.StorageUsed() + sequenceNumberPayloadSize
	return a.setAccountStatusStorageUsed(newAddress, status, storageUsed)
}

func (a *StatefulAccounts) GetAccountPublicKey(
	address flow.Address,
	keyIndex uint32,
) (
	flow.AccountPublicKey,
	error,
) {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	if keyIndex == 0 {
		key, err := getAccountPublicKey0(a, address)
		if err != nil {
			return flow.AccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
		}
		return key, nil
	}

	status, err := a.getAccountStatus(address)
	if err != nil {
		return flow.AccountPublicKey{}, err
	}

	// Get account public key metadata.
	weight, revoked, storedKeyIndex, err := status.AccountPublicKeyMetadata(keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
	}

	// Get stored public key.
	storedKey, err := getStoredPublicKey(a, address, storedKeyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
	}

	// Get sequence number.
	sequenceNumber, err := getAccountPublicKeySequenceNumber(a, address, keyIndex)
	if err != nil {
		return flow.AccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
	}

	return flow.AccountPublicKey{
		Index:     keyIndex,
		PublicKey: storedKey.PublicKey,
		SignAlgo:  storedKey.SignAlgo,
		HashAlgo:  storedKey.HashAlgo,
		SeqNumber: sequenceNumber,
		Weight:    int(weight),
		Revoked:   revoked,
	}, nil
}

func (a *StatefulAccounts) GetRuntimeAccountPublicKey(
	address flow.Address,
	keyIndex uint32,
) (
	flow.RuntimeAccountPublicKey,
	error,
) {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return flow.RuntimeAccountPublicKey{}, err
	}

	if keyIndex == 0 {
		key, err := getAccountPublicKey0(a, address)
		if err != nil {
			return flow.RuntimeAccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
		}
		return flow.RuntimeAccountPublicKey{
			Index:     keyIndex,
			PublicKey: key.PublicKey,
			SignAlgo:  key.SignAlgo,
			HashAlgo:  key.HashAlgo,
			Weight:    key.Weight,
			Revoked:   key.Revoked,
		}, nil
	}

	status, err := a.getAccountStatus(address)
	if err != nil {
		return flow.RuntimeAccountPublicKey{}, err
	}

	// Get account public key metadata.
	weight, revoked, storedKeyIndex, err := status.AccountPublicKeyMetadata(keyIndex)
	if err != nil {
		return flow.RuntimeAccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
	}

	// Get stored public key.
	storedKey, err := getStoredPublicKey(a, address, storedKeyIndex)
	if err != nil {
		return flow.RuntimeAccountPublicKey{}, fmt.Errorf("failed to get account public key at index %d for %s: %w", keyIndex, address, err)
	}

	return flow.RuntimeAccountPublicKey{
		Index:     keyIndex,
		PublicKey: storedKey.PublicKey,
		SignAlgo:  storedKey.SignAlgo,
		HashAlgo:  storedKey.HashAlgo,
		Weight:    int(weight),
		Revoked:   revoked,
	}, nil
}

func (a *StatefulAccounts) GetAccountPublicKeyRevokedStatus(address flow.Address, keyIndex uint32) (bool, error) {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return false, err
	}

	if keyIndex == 0 {
		accountPublicKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return false, err
		}
		return accountPublicKey.Revoked, nil
	}

	status, err := a.getAccountStatus(address)
	if err != nil {
		return false, err
	}

	return status.AccountPublicKeyRevokedStatus(keyIndex)
}

func (a *StatefulAccounts) RevokeAccountPublicKey(
	address flow.Address,
	keyIndex uint32,
) error {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return err
	}

	if keyIndex == 0 {
		return revokeAccountPublicKey0(a, address)
	}

	status, err := a.getAccountStatus(address)
	if err != nil {
		return err
	}

	err = status.RevokeAccountPublicKey(keyIndex)
	if err != nil {
		return fmt.Errorf("failed to revoke public key at index %d for %s: %w", keyIndex, address, err)
	}

	return a.setAccountStatusAfterAccountStatusSizeChange(address, status)
}

func (a *StatefulAccounts) GetAccountPublicKeySequenceNumber(address flow.Address, keyIndex uint32) (uint64, error) {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return 0, err
	}

	if keyIndex == 0 {
		accountPublicKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return 0, err
		}
		return accountPublicKey.SeqNumber, nil
	}

	return getAccountPublicKeySequenceNumber(a, address, keyIndex)
}

func (a *StatefulAccounts) IncrementAccountPublicKeySequenceNumber(address flow.Address, keyIndex uint32) error {
	err := a.accountPublicKeyIndexInRange(address, keyIndex)
	if err != nil {
		return err
	}

	if keyIndex == 0 {
		accountPublicKey, err := getAccountPublicKey0(a, address)
		if err != nil {
			return err
		}
		accountPublicKey.SeqNumber++
		return setAccountPublicKey0(a, address, accountPublicKey)
	}

	return incrementAccountPublicKeySequenceNumber(a, address, keyIndex)
}

func (a *StatefulAccounts) GetAccountPublicKeyCount(
	address flow.Address,
) (
	uint32,
	error,
) {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return 0, fmt.Errorf("failed to get public key count: %w", err)
	}
	return status.AccountPublicKeyCount(), nil
}

func (a *StatefulAccounts) setPublicKeyCount(
	address flow.Address,
	count uint32,
) error {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return fmt.Errorf(
			"failed to set public key count for account (%s): %w",
			address.String(),
			err)
	}

	status.SetAccountPublicKeyCount(count)

	err = a.setAccountStatus(address, status)
	if err != nil {
		return fmt.Errorf(
			"failed to set public key count for account (%s): %w",
			address.String(),
			err)
	}
	return nil
}

func (a *StatefulAccounts) GetAccountPublicKeys(
	address flow.Address,
) (
	publicKeys []flow.AccountPublicKey,
	err error,
) {
	count, err := a.GetAccountPublicKeyCount(address)
	if err != nil {
		return nil, err
	}
	publicKeys = make([]flow.AccountPublicKey, count)

	for i := uint32(0); i < count; i++ {
		publicKey, err := a.GetAccountPublicKey(address, i)
		if err != nil {
			return nil, err
		}

		publicKeys[i] = publicKey
	}

	return publicKeys, nil
}

func (a *StatefulAccounts) AppendAccountPublicKey(
	address flow.Address,
	publicKey flow.AccountPublicKey,
) error {
	count, err := a.GetAccountPublicKeyCount(address)
	if err != nil {
		return err
	}

	newCount := count + 1
	if count >= MaxPublicKeyCount {
		return errors.NewAccountPublicKeyLimitError(
			address,
			newCount,
			MaxPublicKeyCount)
	}

	keyIndex := count

	err = a.appendPublicKey(address, publicKey, keyIndex)
	if err != nil {
		return err
	}

	// Adjust storage used for pre-allocated sequence number for key at index > 0.
	if keyIndex > 0 {
		sequenceNumberPayloadSize := PredefinedSequenceNumberPayloadSize(address, keyIndex)

		status, err := a.getAccountStatus(address)
		if err != nil {
			return err
		}

		storageUsed := status.StorageUsed() + sequenceNumberPayloadSize
		return a.setAccountStatusStorageUsed(address, status, storageUsed)
	}

	return nil
}

func (a *StatefulAccounts) appendPublicKey(
	address flow.Address,
	publicKey flow.AccountPublicKey,
	keyIndex uint32,
) error {
	if !IsValidAccountKeyHashAlgo(publicKey.HashAlgo) {
		return errors.NewValueErrorf(
			publicKey.HashAlgo.String(),
			"hashing algorithm type not found")
	}

	if !IsValidAccountKeySignAlgo(publicKey.SignAlgo) {
		return errors.NewValueErrorf(
			publicKey.SignAlgo.String(),
			"signature algorithm type not found")
	}

	if keyIndex == 0 {
		// Create account public key register for account public key at key index 0
		publicKey.Index = keyIndex
		err := setAccountPublicKey0(a, address, publicKey)
		if err != nil {
			return err
		}

		return a.setPublicKeyCount(address, keyIndex+1)
	}

	storedKey := flow.StoredPublicKey{
		PublicKey: publicKey.PublicKey,
		SignAlgo:  publicKey.SignAlgo,
		HashAlgo:  publicKey.HashAlgo,
	}

	encodedKey, err := flow.EncodeStoredPublicKey(storedKey)
	if err != nil {
		return err
	}

	storedKeyIndex, saveKey, err := a.appendKeyMetadataToAccountStatusRegister(
		address,
		publicKey.Revoked,
		uint16(publicKey.Weight),
		encodedKey,
	)
	if err != nil {
		return err
	}

	// Store key if needed.
	if saveKey {
		err = appendStoredKey(a, address, storedKeyIndex, encodedKey)
		if err != nil {
			return err
		}
	}

	// Store sequence number if needed.
	if publicKey.SeqNumber > 0 {
		err = createAccountPublicKeySequenceNumber(a, address, keyIndex, publicKey.SeqNumber)
		if err != nil {
			return err
		}
	}

	return nil
}

func (a *StatefulAccounts) appendKeyMetadataToAccountStatusRegister(
	address flow.Address,
	revoked bool,
	weight uint16,
	encodedKey []byte,
) (storedKeyIndex uint32, saveKey bool, _ error) {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return 0, false, err
	}

	storedKeyIndex, saveKey, err = status.AppendAccountPublicKeyMetadata(
		revoked,
		weight,
		encodedKey,
		func(b []byte) uint64 {
			return accountkeymetadata.GetPublicKeyDigest(address, b)
		},
		func(storedKeyIndex uint32) ([]byte, error) {
			return getRawStoredPublicKey(a, address, storedKeyIndex)
		},
	)
	if err != nil {
		return 0, false, err
	}

	err = a.setAccountStatusAfterAccountStatusSizeChange(address, status)
	if err != nil {
		return 0, false, err
	}

	return storedKeyIndex, saveKey, nil
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

func (a *StatefulAccounts) getContract(
	contractName string,
	address flow.Address,
) (
	[]byte,
	error,
) {
	contract, err := a.GetValue(flow.ContractRegisterID(address, contractName))
	if err != nil {
		return nil, err
	}

	return contract, nil
}

func (a *StatefulAccounts) setContract(
	contractName string,
	address flow.Address,
	contract []byte,
) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return errors.NewAccountNotFoundError(address)
	}

	id := flow.ContractRegisterID(address, contractName)
	prevContract, err := a.GetValue(id)
	if err != nil {
		return errors.NewContractNotFoundError(address, contractName)
	}

	// skip updating if the new contract equals the old
	if bytes.Equal(prevContract, contract) {
		return nil
	}

	err = a.SetValue(id, contract)
	if err != nil {
		return err
	}

	return nil
}

func EncodeContractNames(contractNames contractNames) ([]byte, error) {
	var buf bytes.Buffer
	cborEncoder := cbor.NewEncoder(&buf)
	err := cborEncoder.Encode(contractNames)
	if err != nil {
		return nil, errors.NewEncodingFailuref(
			err,
			"cannot encode contract names: %s",
			contractNames,
		)
	}
	return buf.Bytes(), nil
}

func (a *StatefulAccounts) setContractNames(
	contractNames contractNames,
	address flow.Address,
) error {
	ok, err := a.Exists(address)
	if err != nil {
		return err
	}

	if !ok {
		return errors.NewAccountNotFoundError(address)
	}

	newContractNames, err := EncodeContractNames(contractNames)
	if err != nil {
		return err
	}

	id := flow.ContractNamesRegisterID(address)
	prevContractNames, err := a.GetValue(id)
	if err != nil {
		return fmt.Errorf("cannot retrieve current contract names: %w", err)
	}

	// skip updating if the new contract names equal the old
	if bytes.Equal(prevContractNames, newContractNames) {
		return nil
	}

	return a.SetValue(id, newContractNames)
}

// GetStorageUsed returns the amount of storage used in bytes by this account
func (a *StatefulAccounts) GetStorageUsed(
	address flow.Address,
) (
	uint64,
	error,
) {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage used: %w", err)
	}
	return status.StorageUsed(), nil
}

func (a *StatefulAccounts) setStorageUsed(
	address flow.Address,
	used uint64,
) error {
	status, err := a.getAccountStatus(address)
	if err != nil {
		return fmt.Errorf("failed to set storage used: %w", err)
	}

	return a.setAccountStatusStorageUsed(address, status, used)
}

func (a *StatefulAccounts) setAccountStatusStorageUsed(
	address flow.Address,
	status *AccountStatus,
	newUsed uint64,
) error {
	status.SetStorageUsed(newUsed)

	err := a.setAccountStatus(address, status)
	if err != nil {
		return fmt.Errorf("failed to set storage used: %w", err)
	}
	return nil
}

func (a *StatefulAccounts) GetValue(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return a.txnState.Get(id)
}

// SetValue sets a value in address' storage
func (a *StatefulAccounts) SetValue(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	err := a.updateRegisterSizeChange(id, value)
	if err != nil {
		return fmt.Errorf("failed to update storage for %s: %w", id, err)
	}
	return a.txnState.Set(id, value)
}

func (a *StatefulAccounts) updateRegisterSizeChange(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	if id.Key == flow.AccountStatusKey {
		// size of this register is always fixed size
		// don't double check this to save time and prevent recursion
		return nil
	}
	if strings.HasPrefix(id.Key, flow.SequenceNumberRegisterKeyPrefix) {
		// Size of sequence number register is included when account public key is appended.
		// Don't double count sequence number registers.
		return nil
	}
	oldValue, err := a.GetValue(id)
	if err != nil {
		return err
	}

	sizeChange := int64(RegisterSize(id, value) - RegisterSize(id, oldValue))
	if sizeChange == 0 {
		// register size has not changed. Nothing to do
		return nil
	}

	address := flow.BytesToAddress([]byte(id.Owner))
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
			return fmt.Errorf("storage would be negative for %s", id)
		}
		newSize = oldSize - absChange
	} else {
		absChange := uint64(sizeChange)
		newSize = oldSize + absChange
	}

	// this puts us back in the setValue method.
	// The difference is that storage_used update exits early from this
	// function so there isn't any recursion.
	return a.setStorageUsed(address, newSize)
}

func RegisterSize(id flow.RegisterID, value flow.RegisterValue) int {
	// NOTE: RegisterSize() needs to be in sync with encodedKeyLength() in ledger/trie_encoder.go.
	if len(value) == 0 {
		// registers with empty value won't (or don't) exist when stored
		return 0
	}
	size := 2 // number of key parts (2 bytes)
	// Size for each key part:
	// length prefix (4 bytes) + encoded key part type (2 bytes) + key part value
	size += 4 + 2 + len(id.Owner)
	size += 4 + 2 + len(id.Key)
	size += len(value)
	return size
}

// GetContractNames gets a sorted list of names of contracts deployed on an
// address
func (a *StatefulAccounts) GetContractNames(
	address flow.Address,
) (
	[]string,
	error,
) {
	return a.getContractNames(address)
}

func (a *StatefulAccounts) getContractNames(
	address flow.Address,
) (
	contractNames,
	error,
) {
	// TODO return fatal error if can't fetch
	encodedContractNames, err := a.GetValue(flow.ContractNamesRegisterID(address))
	if err != nil {
		return nil, fmt.Errorf("cannot get deployed contract names: %w", err)
	}

	return DecodeContractNames(encodedContractNames)
}

func DecodeContractNames(encodedContractNames []byte) ([]string, error) {
	identifiers := make([]string, 0)
	if len(encodedContractNames) > 0 {
		buf := bytes.NewReader(encodedContractNames)
		cborDecoder := cbor.NewDecoder(buf)
		err := cborDecoder.Decode(&identifiers)
		if err != nil {
			return nil, fmt.Errorf(
				"cannot decode deployed contract names %x: %w",
				encodedContractNames,
				err,
			)
		}
	}
	return identifiers, nil
}

func (a *StatefulAccounts) ContractExists(
	contractName string,
	address flow.Address,
) (
	bool,
	error,
) {
	contractNames, err := a.getContractNames(address)
	if err != nil {
		return false, err
	}
	return contractNames.Has(contractName), nil
}

func (a *StatefulAccounts) GetContract(
	contractName string,
	address flow.Address,
) (
	[]byte,
	error,
) {
	// we optimized the happy case here, we look up for the content of the
	// contract and if its not there we check if contract exists or if this is
	// another problem.
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

func (a *StatefulAccounts) SetContract(
	contractName string,
	address flow.Address,
	contract []byte,
) error {
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

func (a *StatefulAccounts) DeleteContract(
	contractName string,
	address flow.Address,
) error {
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

// GenerateAccountLocalID generates a new account local id for an address
// it is sequential and starts at 1
// Errors can happen if the account state cannot be read or written to
func (a *StatefulAccounts) GenerateAccountLocalID(
	address flow.Address,
) (
	uint64,
	error,
) {
	as, err := a.getAccountStatus(address)
	if err != nil {
		return 0, fmt.Errorf("failed to get account local id: %w", err)
	}
	id := as.AccountIdCounter()
	// AccountLocalIDs are defined as non 0 so return the incremented value
	// see: https://github.com/onflow/cadence/blob/2081a601106baaf6ae695e3f2a84613160bb2166/runtime/interface.go#L149
	id += 1

	as.SetAccountIdCounter(id)
	err = a.setAccountStatus(address, as)
	if err != nil {
		return 0, fmt.Errorf("failed to get increment account local id: %w", err)
	}
	return id, nil
}

func (a *StatefulAccounts) getAccountStatus(
	address flow.Address,
) (
	*AccountStatus,
	error,
) {
	id := flow.AccountStatusRegisterID(address)
	statusBytes, err := a.GetValue(id)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to load account status for the account (%s): %w",
			address.String(),
			err)
	}
	if len(statusBytes) == 0 {
		return nil, errors.NewAccountNotFoundError(address)
	}
	return AccountStatusFromBytes(statusBytes)
}

func (a *StatefulAccounts) setAccountStatus(
	address flow.Address,
	status *AccountStatus,
) error {
	id := flow.AccountStatusRegisterID(address)
	err := a.SetValue(id, status.ToBytes())
	if err != nil {
		return fmt.Errorf(
			"failed to store the account status for account (%s): %w",
			address.String(),
			err)
	}
	return nil
}

func (a *StatefulAccounts) accountPublicKeyIndexInRange(
	address flow.Address,
	keyIndex uint32,
) error {
	publicKeyCount, err := a.GetAccountPublicKeyCount(address)
	if err != nil {
		return errors.NewAccountPublicKeyNotFoundError(
			address,
			keyIndex)
	}

	if keyIndex >= publicKeyCount {
		return errors.NewAccountPublicKeyNotFoundError(
			address,
			keyIndex)
	}

	return nil
}

// setAccountStatusAfterAccountStatusSizeChange adjusts and sets
// account storage used after the account status register size is changed.
// This function is needed because updateRegisterSizeChange() filters out
// account status register to prevent recursion when computing storage used,
// so we need to explicitly update the account storage used when the
// account status register size is changed.
func (a *StatefulAccounts) setAccountStatusAfterAccountStatusSizeChange(
	address flow.Address,
	status *AccountStatus,
) error {
	id := flow.AccountStatusRegisterID(address)

	oldAccountStatusValue, err := a.GetValue(id)
	if err != nil {
		return err
	}
	oldAccountStatusSize := len(oldAccountStatusValue)

	newAccountStatusValue := status.ToBytes()
	newAccountStatusSize := len(newAccountStatusValue)

	sizeChange := newAccountStatusSize - oldAccountStatusSize
	if sizeChange == 0 {
		// Account status register size has not changed.

		// Set account status in underlying state
		return a.txnState.Set(id, newAccountStatusValue)
	}

	oldAccountStatus, err := AccountStatusFromBytes(oldAccountStatusValue)
	if err != nil {
		return err
	}
	oldStorageUsed := oldAccountStatus.StorageUsed()

	// Two paths to avoid casting uint to int
	var newStorageUsed uint64
	if sizeChange < 0 {
		absChange := uint64(-sizeChange)
		if absChange > uint64(oldAccountStatusSize) {
			// should never happen
			return fmt.Errorf("storage would be negative for %s", id)
		}
		newStorageUsed = oldStorageUsed - absChange
	} else {
		absChange := uint64(sizeChange)
		newStorageUsed = oldStorageUsed + absChange
	}

	// Set updated storage used
	status.SetStorageUsed(newStorageUsed)

	// Set account status in underlying state
	return a.txnState.Set(id, status.ToBytes())
}

// PredefinedSequenceNumberPayloadSize returns sequence number register size.
func PredefinedSequenceNumberPayloadSize(address flow.Address, keyIndex uint32) uint64 {
	// NOTE: We use 1 byte for register value size as predefined value size.
	sequenceNumberValueUsedForStorageSizeComputation := []byte{0x01}
	size := RegisterSize(
		flow.AccountPublicKeySequenceNumberRegisterID(address, keyIndex),
		sequenceNumberValueUsedForStorageSizeComputation,
	)
	return uint64(size)
}

// contractNames container for a list of contract names. Should always be
// sorted. To ensure this, don't sort while reading it from storage, but sort
// it while adding/removing elements
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
