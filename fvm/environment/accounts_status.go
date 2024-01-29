package environment

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
)

const (
	flagSize             = 1
	storageUsedSize      = 8
	storageIndexSize     = 8
	publicKeyCountsSize  = 8
	addressIdCounterSize = 8

	// oldAccountStatusSize is the size of the account status before the address
	// id counter was added. After v0.32.0 check if it can be removed as all accounts
	// should then have the new status sile len.
	oldAccountStatusSize = flagSize +
		storageUsedSize +
		storageIndexSize +
		publicKeyCountsSize

	accountStatusSize = flagSize +
		storageUsedSize +
		storageIndexSize +
		publicKeyCountsSize +
		addressIdCounterSize

	flagIndex                  = 0
	storageUsedStartIndex      = flagIndex + flagSize
	storageIndexStartIndex     = storageUsedStartIndex + storageUsedSize
	publicKeyCountsStartIndex  = storageIndexStartIndex + storageIndexSize
	addressIdCounterStartIndex = publicKeyCountsStartIndex + publicKeyCountsSize
)

// AccountStatus holds meta information about an account
//
// currently modelled as a byte array with on-demand encoding/decoding of sub arrays
// the first byte captures flags
// the next 8 bytes (big-endian) captures storage used by an account
// the next 8 bytes (big-endian) captures the storage index of an account
// the next 8 bytes (big-endian) captures the number of public keys stored on this account
// the next 8 bytes (big-endian) captures the current address id counter
type AccountStatus [accountStatusSize]byte

// NewAccountStatus returns a new AccountStatus
// sets the storage index to the init value
func NewAccountStatus() *AccountStatus {
	return &AccountStatus{
		0,                      // initial empty flags
		0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
		0, 0, 0, 0, 0, 0, 0, 0, // init value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
	}
}

// ToBytes converts AccountStatus to a byte slice
//
// this has been kept this way in case one day
// we decided to move on to use a struct to represent
// account status.
func (a *AccountStatus) ToBytes() []byte {
	return a[:]
}

// AccountStatusFromBytes constructs an AccountStatus from the given byte slice
func AccountStatusFromBytes(inp []byte) (*AccountStatus, error) {
	var as AccountStatus

	if len(inp) == oldAccountStatusSize {
		// pad the input with zeros
		// this is to migrate old account status to new account status on the fly
		// TODO: remove this whole block after v0.32.0, when a full migration will
		// be made.
		sizeIncrease := uint64(accountStatusSize - oldAccountStatusSize)

		// But we also need to fix the storage used by the appropriate size because
		// the storage used is part of the account status itself.
		copy(as[:], inp)
		used := as.StorageUsed()
		as.SetStorageUsed(used + sizeIncrease)
		return &as, nil
	}

	if len(inp) != accountStatusSize {
		return &as, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account status size")
	}
	copy(as[:], inp)
	return &as, nil
}

// SetStorageUsed updates the storage used by the account
func (a *AccountStatus) SetStorageUsed(used uint64) {
	binary.BigEndian.PutUint64(a[storageUsedStartIndex:storageUsedStartIndex+storageUsedSize], used)
}

// StorageUsed returns the storage used by the account
func (a *AccountStatus) StorageUsed() uint64 {
	return binary.BigEndian.Uint64(a[storageUsedStartIndex : storageUsedStartIndex+storageUsedSize])
}

// SetStorageIndex updates the storage index of the account
func (a *AccountStatus) SetStorageIndex(index atree.SlabIndex) {
	copy(a[storageIndexStartIndex:storageIndexStartIndex+storageIndexSize], index[:storageIndexSize])
}

// StorageIndex returns the storage index of the account
func (a *AccountStatus) StorageIndex() atree.SlabIndex {
	var index atree.SlabIndex
	copy(index[:], a[storageIndexStartIndex:storageIndexStartIndex+storageIndexSize])
	return index
}

// SetPublicKeyCount updates the public key count of the account
func (a *AccountStatus) SetPublicKeyCount(count uint64) {
	binary.BigEndian.PutUint64(a[publicKeyCountsStartIndex:publicKeyCountsStartIndex+publicKeyCountsSize], count)
}

// PublicKeyCount returns the public key count of the account
func (a *AccountStatus) PublicKeyCount() uint64 {
	return binary.BigEndian.Uint64(a[publicKeyCountsStartIndex : publicKeyCountsStartIndex+publicKeyCountsSize])
}

// SetAccountIdCounter updates id counter of the account
func (a *AccountStatus) SetAccountIdCounter(id uint64) {
	binary.BigEndian.PutUint64(a[addressIdCounterStartIndex:addressIdCounterStartIndex+addressIdCounterSize], id)
}

// AccountIdCounter returns id counter of the account
func (a *AccountStatus) AccountIdCounter() uint64 {
	return binary.BigEndian.Uint64(a[addressIdCounterStartIndex : addressIdCounterStartIndex+addressIdCounterSize])
}
