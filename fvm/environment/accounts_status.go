package environment

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/onflow/atree"

	accountkeymetadata "github.com/onflow/flow-go/fvm/environment/account-key-metadata"
	"github.com/onflow/flow-go/fvm/errors"
)

const (
	flagSize                      = 1
	storageUsedSize               = 8
	storageIndexSize              = 8
	oldAccountPublicKeyCountsSize = 8
	accountPublicKeyCountsSize    = 4
	addressIdCounterSize          = 8

	// accountStatusSizeV1 is the size of the account status before the address
	// id counter was added. After Crescendo check if it can be removed as all accounts
	// should then have the new status sile len.
	accountStatusSizeV1 = flagSize +
		storageUsedSize +
		storageIndexSize +
		oldAccountPublicKeyCountsSize

	// accountStatusSizeV2 is the size of the account status before
	// the public key count was changed from 8 to 4 bytes long.
	// After Crescendo check if it can be removed as all accounts
	// should then have the new status sile len.
	accountStatusSizeV2 = flagSize +
		storageUsedSize +
		storageIndexSize +
		oldAccountPublicKeyCountsSize +
		addressIdCounterSize

	accountStatusSizeV3 = flagSize +
		storageUsedSize +
		storageIndexSize +
		accountPublicKeyCountsSize +
		addressIdCounterSize

	flagIndex                        = 0
	storageUsedStartIndex            = flagIndex + flagSize
	storageIndexStartIndex           = storageUsedStartIndex + storageUsedSize
	accountPublicKeyCountsStartIndex = storageIndexStartIndex + storageIndexSize
	addressIdCounterStartIndex       = accountPublicKeyCountsStartIndex + accountPublicKeyCountsSize

	versionMask           = 0xf0
	deduplicationFlagMask = 0x01

	accountStatusV4DefaultVersionAndFlag = 0x40

	AccountStatusMinSizeV4 = accountStatusSizeV3
)

const (
	maxStoredDigests = 2 // Account status register stores up to 2 digests from last 2 stored keys.
)

// AccountStatus holds meta information about an account
//
// currently modelled as a byte array with on-demand encoding/decoding of sub arrays
// the first byte captures flags
// the next 8 bytes (big-endian) captures storage used by an account
// the next 8 bytes (big-endian) captures the storage index of an account
// the next 4 bytes (big-endian) captures the number of public keys stored on this account
// the next 8 bytes (big-endian) captures the current address id counter
type accountStatusV3 [accountStatusSizeV3]byte

type AccountStatus struct {
	accountStatusV3
	keyMetadataBytes []byte
}

// NewAccountStatus returns a new AccountStatus
// sets the storage index to the init value
func NewAccountStatus() *AccountStatus {
	as := accountStatusV3{
		accountStatusV4DefaultVersionAndFlag, // initial empty flags
		0, 0, 0, 0, 0, 0, 0, 0,               // init value for storage used
		0, 0, 0, 0, 0, 0, 0, 1, // init value for storage index
		0, 0, 0, 0, // init value for public key counts
		0, 0, 0, 0, 0, 0, 0, 0, // init value for address id counter
	}
	return &AccountStatus{
		accountStatusV3: as,
	}
}

// ToBytes converts AccountStatus to a byte slice
//
// this has been kept this way in case one day
// we decided to move on to use a struct to represent
// account status.
func (a *AccountStatus) ToBytes() []byte {
	if len(a.keyMetadataBytes) == 0 {
		return a.accountStatusV3[:]
	}
	return append(a.accountStatusV3[:], a.keyMetadataBytes...)
}

// AccountStatusFromBytes constructs an AccountStatus from the given byte slice
func AccountStatusFromBytes(inp []byte) (*AccountStatus, error) {
	asv3, rest, err := accountStatusV3FromBytes(inp)
	if err != nil {
		return nil, err
	}

	// NOTE: both accountStatusV3 and keyMetadataBytes are copies.
	return &AccountStatus{
		accountStatusV3:  asv3,
		keyMetadataBytes: append([]byte(nil), rest...),
	}, nil
}

func accountStatusV3FromBytes(inp []byte) (accountStatusV3, []byte, error) {
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
		// copy the old account status first, so that we don't slice the input
		copy(inp2, inp)

		// cut leading 4 bytes of old public key count.
		cutStart := flagSize +
			storageUsedSize +
			storageIndexSize

		cutEnd := flagSize +
			storageUsedSize +
			storageIndexSize +
			(oldAccountPublicKeyCountsSize - accountPublicKeyCountsSize)

		// check if the public key count is larger than 4 bytes
		for i := cutStart; i < cutEnd; i++ {
			if inp2[i] != 0 {
				return accountStatusV3{}, nil, fmt.Errorf("cannot migrate account status from v2 to v3: public key count is larger than 4 bytes %v, %v", hex.EncodeToString(inp2[flagSize+
					storageUsedSize+
					storageIndexSize:flagSize+
					storageUsedSize+
					storageIndexSize+
					oldAccountPublicKeyCountsSize]), inp2[i])
			}
		}

		inp2 = append(inp2[:cutStart], inp2[cutEnd:]...)

		sizeDecrease := int64(accountStatusSizeV2 - accountStatusSizeV3)

		// But we also need to fix the storage used by the appropriate size because
		// the storage used is part of the account status itself.
		sizeChange -= sizeDecrease

		inp = inp2
	}

	if len(inp) < accountStatusSizeV3 {
		return accountStatusV3{}, nil, errors.NewValueErrorf(hex.EncodeToString(inp), "invalid account status size")
	}

	inp, rest := inp[:accountStatusSizeV3], inp[accountStatusSizeV3:]

	var as accountStatusV3
	copy(as[:], inp)

	if sizeChange != 0 {
		used := as.StorageUsed()

		if sizeChange < 0 {
			// check if the storage used is smaller than the size change
			if used < uint64(-sizeChange) {
				return accountStatusV3{}, nil, errors.NewValueErrorf(hex.EncodeToString(inp), "account would have negative storage used after migration")
			}

			used = used - uint64(-sizeChange)
		}

		if sizeChange > 0 {
			used = used + uint64(sizeChange)
		}

		as.SetStorageUsed(used)
	}

	return as, rest, nil
}

func (a *accountStatusV3) Version() uint8 {
	return (a[0] & versionMask) >> 4
}

func (a *accountStatusV3) IsAccountKeyDeduplicated() bool {
	return (a[0] & deduplicationFlagMask) != 0
}

func (a *accountStatusV3) setAccountKeyDeduplicationFlag() {
	a[0] |= deduplicationFlagMask
}

// SetStorageUsed updates the storage used by the account
func (a *accountStatusV3) SetStorageUsed(used uint64) {
	binary.BigEndian.PutUint64(a[storageUsedStartIndex:storageUsedStartIndex+storageUsedSize], used)
}

// StorageUsed returns the storage used by the account
func (a *accountStatusV3) StorageUsed() uint64 {
	return binary.BigEndian.Uint64(a[storageUsedStartIndex : storageUsedStartIndex+storageUsedSize])
}

// SetStorageIndex updates the storage index of the account
func (a *accountStatusV3) SetStorageIndex(index atree.SlabIndex) {
	copy(a[storageIndexStartIndex:storageIndexStartIndex+storageIndexSize], index[:storageIndexSize])
}

// SlabIndex returns the storage index of the account
func (a *accountStatusV3) SlabIndex() atree.SlabIndex {
	var index atree.SlabIndex
	copy(index[:], a[storageIndexStartIndex:storageIndexStartIndex+storageIndexSize])
	return index
}

// SetAccountPublicKeyCount updates the account public key count of the account
func (a *accountStatusV3) SetAccountPublicKeyCount(count uint32) {
	binary.BigEndian.PutUint32(a[accountPublicKeyCountsStartIndex:accountPublicKeyCountsStartIndex+accountPublicKeyCountsSize], count)
}

// AccountPublicKeyCount returns the account public key count of the account
func (a *accountStatusV3) AccountPublicKeyCount() uint32 {
	return binary.BigEndian.Uint32(a[accountPublicKeyCountsStartIndex : accountPublicKeyCountsStartIndex+accountPublicKeyCountsSize])
}

// SetAccountIdCounter updates id counter of the account
func (a *accountStatusV3) SetAccountIdCounter(id uint64) {
	binary.BigEndian.PutUint64(a[addressIdCounterStartIndex:addressIdCounterStartIndex+addressIdCounterSize], id)
}

// AccountIdCounter returns id counter of the account
func (a *accountStatusV3) AccountIdCounter() uint64 {
	return binary.BigEndian.Uint64(a[addressIdCounterStartIndex : addressIdCounterStartIndex+addressIdCounterSize])
}

// AccountPublicKeyRevokedStatus returns revoked status of account public key at the given key index stored in key metadata.
// NOTE: To avoid checking keyIndex range repeatedly at different levels, caller must ensure keyIndex > 0 and < AccountPublicKeyCount().
func (a *AccountStatus) AccountPublicKeyRevokedStatus(keyIndex uint32) (bool, error) {
	return accountkeymetadata.GetRevokedStatus(a.keyMetadataBytes, keyIndex)
}

// AccountPublicKeyMetadata returns weight, revoked status, and stored key index of account public key at the given key index stored in key metadata.
// NOTE: To avoid checking keyIndex range repeatedly at different levels, caller must ensure keyIndex > 0 and < AccountPublicKeyCount().
func (a *AccountStatus) AccountPublicKeyMetadata(keyIndex uint32) (
	weight uint16,
	revoked bool,
	storedKeyIndex uint32,
	err error,
) {
	return accountkeymetadata.GetKeyMetadata(a.keyMetadataBytes, keyIndex, a.IsAccountKeyDeduplicated())
}

// RevokeAccountPublicKey revokes account public key at the given key index stored in key metadata.
// NOTE: To avoid checking keyIndex range repeatedly at different levels, caller must ensure keyIndex > 0 and < AccountPublicKeyCount().
func (a *AccountStatus) RevokeAccountPublicKey(keyIndex uint32) error {
	var err error
	a.keyMetadataBytes, err = accountkeymetadata.SetRevokedStatus(a.keyMetadataBytes, keyIndex)
	if err != nil {
		return err
	}

	return nil
}

// AppendAccountPublicKeyMetadata appends and deduplicates account public key metadata.
// NOTE: If AppendAccountPublicKeyMetadata returns true for saveKey, caller is responsible for
// saving the key corresponding to the given key metadata to storage.
func (a *AccountStatus) AppendAccountPublicKeyMetadata(
	revoked bool,
	weight uint16,
	encodedKey []byte,
	getKeyDigest func([]byte) uint64,
	getStoredKey func(uint32) ([]byte, error),
) (storedKeyIndex uint32, saveKey bool, err error) {

	accountPublicKeyCount := a.AccountPublicKeyCount()
	keyIndex := accountPublicKeyCount

	if keyIndex == 0 {
		// First account public key's metadata is not saved in order to
		// reduce storage overhead because most accounts only have one key.

		// Increment public key count.
		a.SetAccountPublicKeyCount(accountPublicKeyCount + 1)
		return 0, true, nil
	}

	var keyMetadata *accountkeymetadata.KeyMetadataAppender
	keyMetadata, storedKeyIndex, saveKey, err = a.appendAccountPublicKeyMetadata(keyIndex, revoked, weight, encodedKey, getKeyDigest, getStoredKey)
	if err != nil {
		return 0, false, err
	}

	// Serialize key metadata and set account duplication flag if needed.
	var deduplicated bool
	a.keyMetadataBytes, deduplicated = keyMetadata.ToBytes()
	if deduplicated {
		a.setAccountKeyDeduplicationFlag()
	}

	a.SetAccountPublicKeyCount(accountPublicKeyCount + 1)

	return storedKeyIndex, saveKey, nil
}

// appendAccountPublicKeyMetadata is the main implementation of AppendAccountPublicKeyMetadata()
// and it should only be called by that function.
func (a *AccountStatus) appendAccountPublicKeyMetadata(
	keyIndex uint32,
	revoked bool,
	weight uint16,
	encodedKey []byte,
	getKeyDigest func([]byte) uint64,
	getStoredKey func(uint32) ([]byte, error),
) (
	_ *accountkeymetadata.KeyMetadataAppender,
	storedKeyIndex uint32,
	saveKey bool,
	err error,
) {

	var keyMetadata *accountkeymetadata.KeyMetadataAppender

	if len(a.keyMetadataBytes) == 0 {
		// New key index must be 1 when key metadata is empty because
		// key metadata at key index 0 is stored with public key at
		// key index 0 in a separate register (apk_0).
		// Most accounts only have 1 account public key and we
		// special case for that as an optimization.

		if keyIndex != 1 {
			return nil, 0, false, errors.NewKeyMetadataEmptyError(fmt.Sprintf("key metadata cannot be empty when appending new key metadata at index %d", keyIndex))
		}

		// To avoid storage overhead for most accounts, account key 0 digest is computed and stored when account key 1 is added.
		// So if new key index is 1, we need to compute and append key 0 digest to keyMetadata before appending key 1 metadata.

		// Get public key 0.
		var key0 []byte
		key0, err = getStoredKey(0)
		if err != nil {
			return nil, 0, false, err
		}

		// Get public key 0 digest.
		key0Digest := getKeyDigest(key0)

		// Create empty KeyMetadataAppender with key 0 digest.
		keyMetadata = accountkeymetadata.NewKeyMetadataAppender(key0Digest, maxStoredDigests)
	} else {
		// Create KeyMetadataAppender with stored key metadata bytes.
		keyMetadata, err = accountkeymetadata.NewKeyMetadataAppenderFromBytes(a.keyMetadataBytes, a.IsAccountKeyDeduplicated(), maxStoredDigests)
		if err != nil {
			return nil, 0, false, err
		}
	}

	digest, isDuplicateKey, duplicateStoredKeyIndex, err := accountkeymetadata.FindDuplicateKey(keyMetadata, encodedKey, getKeyDigest, getStoredKey)
	if err != nil {
		return nil, 0, false, err
	}

	// Whether new public key is a duplicate or not, we store these items in key metadata section:
	// - new account public key's revoked status and weight
	// - new public key's digest (we only store last N digests and N=2 by default to balance tradeoffs)
	// If new public key is a duplicate, we also store mapping of account key index to stored key index.
	//
	// As a non-duplicate key example, if public key at index 1 is unique, we store:
	// - new key's weight and revoked status, and
	// - new key's digest
	//
	// As a duplicate key example, if public key at index 1 is duplicate of public key at index 0, we store:
	// - new key's weight and revoked status,
	// - mapping indicating public key at index 1 is the same as public key at index 0.
	// - new key's digest

	// Handle duplicate key.
	if isDuplicateKey {
		err = keyMetadata.AppendDuplicateKeyMetadata(keyIndex, duplicateStoredKeyIndex, revoked, weight)
		if err != nil {
			return nil, 0, false, err
		}

		return keyMetadata, duplicateStoredKeyIndex, false, nil
	}

	// Handle non-duplicate key.
	storedKeyIndex, err = keyMetadata.AppendUniqueKeyMetadata(revoked, weight, digest)
	if err != nil {
		return nil, 0, false, err
	}

	return keyMetadata, storedKeyIndex, true, nil
}
