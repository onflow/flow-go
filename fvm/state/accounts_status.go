package state

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
)

const (
	flagSize            = 1
	storageUsedSize     = 8
	storageIndexSize    = 8
	publicKeyCountsSize = 8

	accountStatusSize = flagSize +
		storageUsedSize +
		storageIndexSize +
		publicKeyCountsSize

	flagIndex                 = 0
	storageUsedStartIndex     = flagIndex + flagSize
	storageIndexStartIndex    = storageUsedStartIndex + storageUsedSize
	publicKeyCountsStartIndex = storageIndexStartIndex + storageIndexSize
)

// AccountStatus holds meta information about an account
//
// currently modelled as a byte array with on-demand encoding/decoding of sub arrays
// the first byte captures flags (e.g. frozen)
// the next 8 bytes (big-endian) captures storage used by an account
// the next 8 bytes (big-endian) captures the storage index of an account
// and the last 8 bytes (big-endian) captures the number of public keys stored on this account
type AccountStatus [accountStatusSize]byte

const (
	maskFrozen byte = 0b1000_0000
)

// NewAccountStatus returns a new AccountStatus
// sets the storage index to the init value
func NewAccountStatus() *AccountStatus {
	return &AccountStatus{
		0,                      // initial empty flags
		0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
		0, 0, 0, 0, 0, 0, 0, 0, // init value for public key counts
	}
}

// ToBytes converts AccountStatus to a byte slice
//
// this has been kept this way in case one day
// we decided to move on to use an struct to represent
// account status.
func (a *AccountStatus) ToBytes() []byte {
	return a[:]
}

// AccountStatusFromBytes constructs an AccountStatus from the given byte slice
func AccountStatusFromBytes(inp []byte) (*AccountStatus, error) {
	var as AccountStatus
	if len(inp) != accountStatusSize {
		return &as, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account status size")
	}
	copy(as[:], inp)
	return &as, nil
}

// IsAccountFrozen returns true if account's frozen flag is set
func (a *AccountStatus) IsAccountFrozen() bool {
	return a[flagIndex]&maskFrozen > 0
}

// SetFrozenFlag sets the frozen flag
func (a *AccountStatus) SetFrozenFlag(frozen bool) {
	if frozen {
		a[flagIndex] = a[flagIndex] | maskFrozen
		return
	}
	a[flagIndex] = a[flagIndex] & (0xFF - maskFrozen)
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
func (a *AccountStatus) SetStorageIndex(index atree.StorageIndex) {
	copy(a[storageIndexStartIndex:storageIndexStartIndex+storageIndexSize], index[:storageIndexSize])
}

// StorageIndex returns the storage index of the account
func (a *AccountStatus) StorageIndex() atree.StorageIndex {
	var index atree.StorageIndex
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
