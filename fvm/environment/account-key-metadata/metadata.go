package accountkeymetadata

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"slices"

	"github.com/onflow/flow-go/fvm/errors"
)

// GetRevokedStatus returns revoked status for account public key at the key index.
func GetRevokedStatus(b []byte, keyIndex uint32) (bool, error) {
	// Key metadata only stores weight and revoked status for keys at index > 0.

	if keyIndex == 0 {
		return false, errors.NewKeyMetadataUnexpectedKeyIndexError("failed to query revoked status", 0)
	}

	weightAndRevokedStatusBytes, _, err := parseWeightAndRevokedStatusFromKeyMetadataBytes(b)
	if err != nil {
		return false, err
	}

	revoked, _, err := getWeightAndRevokedStatus(weightAndRevokedStatusBytes, keyIndex-1)
	return revoked, err
}

// GetKeyMetadata returns weight, revoked status, and stored key index for given account public key index.
func GetKeyMetadata(b []byte, keyIndex uint32, deduplicated bool) (
	weight uint16,
	revoked bool,
	storedKeyIndex uint32,
	err error,
) {
	// Key metadata only stores weight and revoked status for keys at index > 0.

	if keyIndex == 0 {
		err = errors.NewKeyMetadataUnexpectedKeyIndexError("failed to query key metadata", 0)
		return
	}

	// Get raw weight and revoked status bytes
	weightAndRevokedStatusBytes, rest, err := parseWeightAndRevokedStatusFromKeyMetadataBytes(b)
	if err != nil {
		return 0, false, 0, err
	}

	// Get weight and revoked status for given account key index
	revoked, weight, err = getWeightAndRevokedStatus(weightAndRevokedStatusBytes, keyIndex-1)
	if err != nil {
		return 0, false, 0, err
	}

	// If keys are not deduplicated, storedKeyIndex is the same as the given keyIndex.
	if !deduplicated {
		return weight, revoked, keyIndex, nil
	}

	// Get raw key index mapping bytes
	startIndexForMapping, mappingBytes, _, err := parseStoredKeyMappingFromKeyMetadataBytes(rest)
	if err != nil {
		return 0, false, 0, err
	}

	// StoredKeyIndex is the same as the given keyIndex if deduplication happens afterwards.
	if keyIndex < startIndexForMapping {
		return weight, revoked, keyIndex, nil
	}

	// Get stored key index from mapping
	storedKeyIndex, err = getStoredKeyIndexFromMappings(mappingBytes, keyIndex-startIndexForMapping)
	if err != nil {
		return 0, false, 0, err
	}

	return weight, revoked, storedKeyIndex, nil
}

// SetRevokedStatus revokes key and returns encoded key metadata.
// NOTE: b may be modified.
func SetRevokedStatus(b []byte, keyIndex uint32) ([]byte, error) {
	// Key metadata only stores weight and revoked status for keys at index > 0.

	if keyIndex == 0 {
		return nil, errors.NewKeyMetadataUnexpectedKeyIndexError("failed to set revoked status", 0)
	}

	weightAndRevokedStatusBytes, rest, err := parseWeightAndRevokedStatusFromKeyMetadataBytes(b)
	if err != nil {
		return nil, err
	}

	newWeightAndRevokedStatusBytes, err := setRevokedStatus(slices.Clone(weightAndRevokedStatusBytes), keyIndex-1)
	if err != nil {
		return nil, err
	}

	newB := make([]byte, lengthPrefixSize+len(newWeightAndRevokedStatusBytes)+len(rest))
	off := 0

	binary.BigEndian.PutUint32(newB, uint32(len(newWeightAndRevokedStatusBytes)))
	off += 4

	n := copy(newB[off:], newWeightAndRevokedStatusBytes)
	off += n

	copy(newB[off:], rest)
	return newB, nil
}

type KeyMetadataAppender struct {
	original                    []byte
	weightAndRevokedStatusBytes []byte // copied
	mappingBytes                []byte // copied
	digestBytes                 []byte // copied
	startIndexForMapping        uint32
	startIndexForDigests        uint32
	maxStoredDigests            uint32
	deduplicated                bool
}

func NewKeyMetadataAppender(key0Digest uint64, maxStoredDigests uint32) *KeyMetadataAppender {
	keyMetadata := KeyMetadataAppender{
		maxStoredDigests: maxStoredDigests,
	}
	keyMetadata.appendDigest(key0Digest)
	return &keyMetadata
}

// NewKeyMetadataAppenderFromBytes returns KeyMetadataAppender used to append new key metadata.
// NOTE: b can be modified.
func NewKeyMetadataAppenderFromBytes(b []byte, deduplicated bool, maxStoredDigests uint32) (*KeyMetadataAppender, error) {
	if len(b) == 0 {
		return nil, fmt.Errorf("failed to create KeyMetadataAppender with empty data: use NewKeyMetadataAppend() instead")
	}

	keyMetadata := KeyMetadataAppender{
		original:         b,
		deduplicated:     deduplicated,
		maxStoredDigests: maxStoredDigests,
	}

	var err error

	// Get revoked and weight raw bytes.
	var weightAndRevokedStatusBytes []byte
	weightAndRevokedStatusBytes, b, err = parseWeightAndRevokedStatusFromKeyMetadataBytes(b)
	if err != nil {
		return nil, err
	}
	keyMetadata.weightAndRevokedStatusBytes = slices.Clone(weightAndRevokedStatusBytes)

	// Get mapping raw bytes.
	if deduplicated {
		var mappingBytes []byte
		keyMetadata.startIndexForMapping, mappingBytes, b, err = parseStoredKeyMappingFromKeyMetadataBytes(b)
		if err != nil {
			return nil, err
		}
		keyMetadata.mappingBytes = slices.Clone(mappingBytes)
	}

	// Get digests list
	var digestBytes []byte
	keyMetadata.startIndexForDigests, digestBytes, b, err = parseDigestsFromKeyMetadataBytes(b)
	if err != nil {
		return nil, err
	}
	keyMetadata.digestBytes = slices.Clone(digestBytes)

	if len(b) != 0 {
		return nil,
			errors.NewKeyMetadataTrailingDataError(
				"failed to parse key metadata",
				len(b),
			)
	}

	return &keyMetadata, nil
}

// With deduplicated flag, account key metadata is encoded as:
// - length prefixed list of account public key weight and revoked status starting from key index 1
// - startKeyIndex (4 bytes) + length prefixed list of account public key index mappings to stored key index
// - startStoredKeyIndex (4 bytes) + length prefixed list of last N stored key digests
//
// Without deduplicated flag, account key metadata is encoded as:
// - length prefixed list of account public key weight and revoked status starting from key index 1
// - startStoredKeyIndex (4 bytes) + length prefixed list of last N stored key digests
func (m *KeyMetadataAppender) ToBytes() ([]byte, bool) {
	size := lengthPrefixSize + len(m.weightAndRevokedStatusBytes) + // length prefixed weight and revoked status
		storedKeyIndexSize + // start stored key index
		lengthPrefixSize + len(m.digestBytes) // length prefixed digests

	if m.deduplicated {
		size += storedKeyIndexSize + // start key index
			lengthPrefixSize + len(m.mappingBytes) // length prefixed mappings
	}

	var b []byte
	if cap(m.original) >= size {
		b = m.original[:size]
	} else {
		b = make([]byte, size)
	}

	off := 0

	// Encode length of encoded weight and revoked status
	binary.BigEndian.PutUint32(b[off:], uint32(len(m.weightAndRevokedStatusBytes)))
	off += 4

	// Copy encoded weight and revoked status
	n := copy(b[off:], m.weightAndRevokedStatusBytes)
	off += n

	// Encode mapping if deduplication is on
	if m.deduplicated {
		// Encode account public key start index for mapping
		binary.BigEndian.PutUint32(b[off:], m.startIndexForMapping)
		off += 4

		// Encode length of encoded mapping
		binary.BigEndian.PutUint32(b[off:], uint32(len(m.mappingBytes)))
		off += 4

		// Copy encoded mapping
		n := copy(b[off:], m.mappingBytes)
		off += n
	}

	// Encode digests

	// Encoded start index for digests
	binary.BigEndian.PutUint32(b[off:], m.startIndexForDigests)
	off += 4

	// Encode length of encoded digests
	binary.BigEndian.PutUint32(b[off:], uint32(len(m.digestBytes)))
	off += 4

	// Copy encoded digest
	copy(b[off:], m.digestBytes)

	return b, m.deduplicated
}

func (m *KeyMetadataAppender) appendDigest(digest uint64) {
	digestCount := 1 + len(m.digestBytes)/digestSize

	if digestCount > int(m.maxStoredDigests) {
		// Remove digest from front
		removeCount := digestCount - int(m.maxStoredDigests)
		m.digestBytes = slices.Delete(m.digestBytes, 0, removeCount*digestSize)

		// Adjust digest start index
		m.startIndexForDigests += uint32(removeCount)
	}

	var digestBytes [digestSize]byte
	binary.BigEndian.PutUint64(digestBytes[:], digest)

	m.digestBytes = append(m.digestBytes, digestBytes[:]...)
}

// AppendUniqueKeyMetadata appends unique key metadata.
func (m *KeyMetadataAppender) AppendUniqueKeyMetadata(
	revoked bool,
	weight uint16,
	digest uint64,
) (storedKeyIndex uint32, err error) {

	storedKeyIndex = m.storedKeyCount()

	// Append revoked status and weight
	m.weightAndRevokedStatusBytes, err = appendWeightAndRevokedStatus(m.weightAndRevokedStatusBytes, revoked, weight)
	if err != nil {
		return 0, err
	}

	// Append digest
	m.appendDigest(digest)

	if m.deduplicated {
		// Append next stored key index to mapping
		m.mappingBytes, err = appendStoredKeyIndexToMappings(m.mappingBytes, storedKeyIndex)
		if err != nil {
			return 0, err
		}
	}

	return storedKeyIndex, nil
}

// AppendDuplicateKeyMetadata appends duplicate key metadata.
func (m *KeyMetadataAppender) AppendDuplicateKeyMetadata(
	keyIndex uint32,
	duplicateStoredKeyIndex uint32,
	revoked bool,
	weight uint16,
) (err error) {

	// Append revoked status and weight
	m.weightAndRevokedStatusBytes, err = appendWeightAndRevokedStatus(m.weightAndRevokedStatusBytes, revoked, weight)
	if err != nil {
		return err
	}

	if !m.deduplicated {
		// Set deduplication flag
		m.deduplicated = true

		// Save mapping start key index.
		m.startIndexForMapping = keyIndex
	}

	// Append duplicate stored key index to mapping
	m.mappingBytes, err = appendStoredKeyIndexToMappings(m.mappingBytes, duplicateStoredKeyIndex)
	if err != nil {
		return err
	}

	return nil
}

func (m *KeyMetadataAppender) storedKeyCount() uint32 {
	return m.startIndexForDigests + uint32(len(m.digestBytes)/digestSize)
}

// findDuplicateDigest returns true and stored key index with duplicate digest
// if the given digest has a match in stored digests in key metadata section.
func (m *KeyMetadataAppender) findDuplicateDigest(digest uint64) (found bool, duplicateStoredKeyIndex uint32) {
	if len(m.digestBytes) == 0 {
		return false, 0
	}

	var digestBytes [digestSize]byte
	binary.BigEndian.PutUint64(digestBytes[:], digest)

	for off, i := 0, uint32(0); off < len(m.digestBytes); off, i = off+digestSize, i+1 {
		if bytes.Equal(digestBytes[:], m.digestBytes[off:off+digestSize]) {
			return true, m.startIndexForDigests + i
		}
	}

	return false, 0
}
