package state

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/fvm/errors"
)

const (
	// AccountStatusSize returns the byteslice size of an account status
	AccountStatusSize = 1 + // flags
		8 + // storage used
		8 + // storage index
		8 // public key counts

	storageUsedStartIndex     = 1
	storageIndexStartIndex    = 1 + 8
	publicKeyCountsStartIndex = 1 + 8 + 8
)

// AccountStatus holds meta information about an account
//
// currently modelled as a byte slice with on-demand encoding/decoding of sub parts
// the first byte captures flags (e.g. frozen)
// the next 8 bytes (big-endian) captures storage used by an account
// the next 8 bytes (big-endian) captures the storage index of an account
// and the last 8 bytes (big-endian) captures the number of public keys stored on this account
// if the length of this byte slice is zero, the account doesn't exist
type AccountStatus []byte

const (
	maskFrozen byte = 0b1000_0000
)

// NewAccountStatus sets exist flag and return an AccountStatus
func NewAccountStatus() AccountStatus {
	// it should at least be 1 byte to set as existing account
	a := make([]byte, AccountStatusSize)
	return AccountStatus(a)
}

// ToBytes converts AccountStatus to a byte slice
//
// this has been kept this way in case one day we move
// away from using a byte slice and use an struct instead
// for modelling account status
func (a AccountStatus) ToBytes() []byte {
	return a
}

// AccountStatusFromBytes constructs an AccountStatus from the given byte slice
func AccountStatusFromBytes(inp []byte) (AccountStatus, error) {
	if len(inp) != AccountStatusSize {
		return nil, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account status size")
	}
	return AccountStatus(inp), nil
}

// AccountExists returns true if account exists
func (a AccountStatus) AccountExists() bool {
	return len(a) > 0
}

// IsAccountFrozen returns true if account's frozen flag is set
func (a AccountStatus) IsAccountFrozen() bool {
	return a[0]&maskFrozen > 0
}

// SetFrozenFlag sets the frozen flag
func (a AccountStatus) SetFrozenFlag(frozen bool) {
	if frozen {
		a[0] = a[0] | maskFrozen
		return
	}
	a[0] = a[0] & (0xFF - maskFrozen)
}

// SetStorageUsed updates the storage used by the account
func (a AccountStatus) SetStorageUsed(used uint64) {
	binary.BigEndian.PutUint64(a[storageUsedStartIndex:storageIndexStartIndex], used)
}

// StorageUsed returns the storage used by the account
func (a AccountStatus) StorageUsed() uint64 {
	return binary.BigEndian.Uint64(a[storageUsedStartIndex:storageIndexStartIndex])
}

// SetStorageIndex updates the storage index of the account
func (a AccountStatus) SetStorageIndex(index atree.StorageIndex) {
	copy(a[storageIndexStartIndex:publicKeyCountsStartIndex], index[:8])
}

// StorageIndex returns the storage index of the account
func (a AccountStatus) StorageIndex() atree.StorageIndex {
	var index atree.StorageIndex
	copy(index[:], a[storageIndexStartIndex:publicKeyCountsStartIndex])
	return index
}

// SetPublicKeyCount updates the public key count of the account
func (a AccountStatus) SetPublicKeyCount(count uint64) {
	binary.BigEndian.PutUint64(a[publicKeyCountsStartIndex:], count)
}

// PublicKeyCount returns the public key count of the account
func (a AccountStatus) PublicKeyCount() uint64 {
	return binary.BigEndian.Uint64(a[publicKeyCountsStartIndex:])
}
