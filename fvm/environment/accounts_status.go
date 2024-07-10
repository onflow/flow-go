package environment

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/onflow/atree"

	"github.com/onflow/flow-go/fvm/errors"
)

const (
	flagSize               = 1
	storageUsedSize        = 8
	storageIndexSize       = 8
	oldPublicKeyCountsSize = 8
	publicKeyCountsSize    = 4
	addressIdCounterSize   = 8

	// accountStatusSizeV1 is the size of the account status before the address
	// id counter was added. After Crescendo check if it can be removed as all accounts
	// should then have the new status sile len.
	accountStatusSizeV1 = flagSize +
		storageUsedSize +
		storageIndexSize +
		oldPublicKeyCountsSize

	// accountStatusSizeV2 is the size of the account status before
	// the public key count was changed from 8 to 4 bytes long.
	// After Crescendo check if it can be removed as all accounts
	// should then have the new status sile len.
	accountStatusSizeV2 = flagSize +
		storageUsedSize +
		storageIndexSize +
		oldPublicKeyCountsSize +
		addressIdCounterSize

	accountStatusSizeV3 = flagSize +
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
// the next 4 bytes (big-endian) captures the number of public keys stored on this account
// the next 8 bytes (big-endian) captures the current address id counter
type AccountStatus [accountStatusSizeV3]byte

// NewAccountStatus returns a new AccountStatus
// sets the storage index to the init value
func NewAccountStatus() *AccountStatus {
	return &AccountStatus{
		0,                      // initial empty flags
		0, 0, 0, 0, 0, 0, 0, 0, // init value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
		0, 0, 0, 0, // init value for public key counts
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
	sizeChange := int64(0)

	// this is to migrate old account status to new account status on the fly
	// TODO: remove this whole block after Crescendo, when a full migration will be made.
	if len(inp) == accountStatusSizeV1 {
		// migrate v1 to v2
		inp2 := make([]byte, accountStatusSizeV2)

		// pad the input with zeros
		sizeIncrease := int64(accountStatusSizeV2 - accountStatusSizeV1)

		// But we also need to fix the storage used by the appropriate size because
		// the storage used is part of the account status itself.
		copy(inp2, inp)
		sizeChange = sizeIncrease

		inp = inp2
	}

	// this is to migrate old account status to new account status on the fly
	// TODO: remove this whole block after Crescendo, when a full migration will be made.
	if len(inp) == accountStatusSizeV2 {
		// migrate v2 to v3

		inp2 := make([]byte, accountStatusSizeV2)
		copy(inp2, inp)

		// cut leading 4 bytes of old public key count.
		cutStart := flagSize +
			storageUsedSize +
			storageIndexSize

		cutEnd := flagSize +
			storageUsedSize +
			storageIndexSize +
			(oldPublicKeyCountsSize - publicKeyCountsSize)

		// check if the public key count is larger than 4 bytes
		for i := cutStart; i < cutEnd; i++ {
			if inp2[i] != 0 {
				return nil, fmt.Errorf("cannot migrate account status from v2 to v3: public key count is larger than 4 bytes %v, %v", hex.EncodeToString(inp2[flagSize+
					storageUsedSize+
					storageIndexSize:flagSize+
					storageUsedSize+
					storageIndexSize+
					oldPublicKeyCountsSize]), inp2[i])
			}
		}

		inp2 = append(inp2[:cutStart], inp2[cutEnd:]...)

		sizeDecrease := int64(accountStatusSizeV2 - accountStatusSizeV3)

		// But we also need to fix the storage used by the appropriate size because
		// the storage used is part of the account status itself.
		sizeChange -= sizeDecrease

		inp = inp2
	}

	var as AccountStatus
	if len(inp) != accountStatusSizeV3 {
		return &as, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account status size")
	}
	copy(as[:], inp)
	if sizeChange != 0 {
		used := as.StorageUsed()

		if sizeChange < 0 {
			// check if the storage used is smaller than the size change
			if used < uint64(-sizeChange) {
				return nil, errors.NewValueErrorf(hex.EncodeToString(inp), "account would have negative storage used after migration")
			}

			used = used - uint64(-sizeChange)
		}

		if sizeChange > 0 {
			used = used + uint64(sizeChange)
		}

		as.SetStorageUsed(used)
	}

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
func (a *AccountStatus) SetPublicKeyCount(count uint32) {
	binary.BigEndian.PutUint32(a[publicKeyCountsStartIndex:publicKeyCountsStartIndex+publicKeyCountsSize], count)
}

// PublicKeyCount returns the public key count of the account
func (a *AccountStatus) PublicKeyCount() uint32 {
	return binary.BigEndian.Uint32(a[publicKeyCountsStartIndex : publicKeyCountsStartIndex+publicKeyCountsSize])
}

// SetAccountIdCounter updates id counter of the account
func (a *AccountStatus) SetAccountIdCounter(id uint64) {
	binary.BigEndian.PutUint64(a[addressIdCounterStartIndex:addressIdCounterStartIndex+addressIdCounterSize], id)
}

// AccountIdCounter returns id counter of the account
func (a *AccountStatus) AccountIdCounter() uint64 {
	return binary.BigEndian.Uint64(a[addressIdCounterStartIndex : addressIdCounterStartIndex+addressIdCounterSize])
}
