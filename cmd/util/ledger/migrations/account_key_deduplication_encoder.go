package migrations

import (
	"encoding/binary"
	"fmt"
	"math"

	accountkeymetadata "github.com/onflow/flow-go/fvm/environment/account-key-metadata"
	"github.com/onflow/flow-go/model/flow"
)

const (
	lengthPrefixSize = 4
	runLengthSize    = 2
)

// Account Public Key Weight and Revoked Status

const (
	// maxRunLengthInEncodedStatusGroup (65535) is the max run length that
	// can be stored in each RLE encoded status group.
	maxRunLengthInEncodedStatusGroup = math.MaxUint16

	// weightAndRevokedStatusSize (2) is the number of bytes used to store
	// the weight and status together as a uint16:
	// - the high bit is the revoked status
	// - the remaining 15 bits is the weight (more than enough for its 0..1000 range)
	weightAndRevokedStatusSize = 2

	// weightAndRevokedStatusGroupSize (4) is the number of bytes used to store
	// the uint16 run length and the uint16 representing weight and revoked status.
	weightAndRevokedStatusGroupSize = runLengthSize + weightAndRevokedStatusSize

	// revokedMask is the bitmask for setting or getting the revoked flag stored
	// as the high bit of a uint16.
	revokedMask = 0x8000

	// weightMask is the bitmask for getting the weight from the low 15 bits (fifteen bits) of
	// the uint16 containing the unsigned 15-bit weight.
	weightMask = 0x7fff
)

type accountPublicKeyWeightAndRevokedStatus struct {
	weight  uint16 // Weight is 0-1000
	revoked bool
}

// accountPublicKeyWeightAndRevokedStatus is encoded using RLE:
// - run length (2 bytes)
// - value (2 bytes): revoked status is the high bit and weight is the remaining 15 bits.
// NOTE: if number of elements in a run-length group exceeds maxRunLengthInEncodedStatusGroup,
// a new group is created with remaining run-length and the same weight and revoked status.
func encodeAccountPublicKeyWeightsAndRevokedStatus(weightsAndRevoked []accountPublicKeyWeightAndRevokedStatus) ([]byte, error) {
	if len(weightsAndRevoked) == 0 {
		return nil, nil
	}

	buf := make([]byte, 0, len(weightsAndRevoked)*(weightAndRevokedStatusGroupSize))

	off := 0
	for i := 0; i < len(weightsAndRevoked); {
		runLength := 1
		value := weightsAndRevoked[i]
		i++

		// Find group boundary
		for i < len(weightsAndRevoked) && runLength < maxRunLengthInEncodedStatusGroup && weightsAndRevoked[i] == value {
			runLength++
			i++
		}

		// Encode weight and revoked status group

		buf = buf[:off+weightAndRevokedStatusGroupSize]

		binary.BigEndian.PutUint16(buf[off:], uint16(runLength))
		off += runLengthSize

		weightAndRevoked := value.weight
		if value.revoked {
			weightAndRevoked |= revokedMask // Turn on high bit for revoked status
		}

		binary.BigEndian.PutUint16(buf[off:], weightAndRevoked)
		off += weightAndRevokedStatusSize
	}

	return buf, nil
}

func decodeAccountPublicKeyWeightAndRevokedStatusGroups(b []byte) ([]accountPublicKeyWeightAndRevokedStatus, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode weight and revoked status: expect multiple of %d bytes, got %d", weightAndRevokedStatusGroupSize, len(b))
	}

	statuses := make([]accountPublicKeyWeightAndRevokedStatus, 0, len(b)/weightAndRevokedStatusGroupSize)

	for i := 0; i < len(b); i += weightAndRevokedStatusGroupSize {
		runLength := uint32(binary.BigEndian.Uint16(b[i:]))
		weightAndRevoked := binary.BigEndian.Uint16(b[i+2 : i+4])

		status := accountPublicKeyWeightAndRevokedStatus{
			weight:  weightAndRevoked & weightMask,
			revoked: (weightAndRevoked & revokedMask) > 0,
		}

		for range runLength {
			statuses = append(statuses, status)
		}
	}

	return statuses, nil
}

// Account Public Key Index to Stored Public Key Index Mappings
const (
	storedKeyIndexSize       = 4
	mappingGroupSize         = runLengthSize + storedKeyIndexSize
	consecutiveGroupFlagMask = 0x8000
	lengthMask               = 0x7fff
)

// encodeAccountPublicKeyMapping encodes keyIndexMappings into concatenated run-length groups.
// Each run-length group is encoded as:
// - length in the low 15 bits of uint16 (2 bytes)
// - stored key index as uint32 (4 bytes)
// For example, account has 8 account keys with 5 unique keys.
// Unique key index mapping is {Key0, Key1, Key1, Key1, Key1, Key2, Key3, Key4}.
// The example's encoded mapping would be:
// { {run-length 1, value 0}, {run-length 4, value 1}, {consecutive-run-length 3, start-value 2}}
func encodeAccountPublicKeyMapping(mapping []uint32) ([]byte, error) {
	if len(mapping) == 0 {
		return nil, nil
	}

	firstGroup := accountkeymetadata.NewMappingGroup(1, mapping[0], false)

	if len(mapping) == 1 {
		return firstGroup.Encode(), nil
	}

	groups := make([]*accountkeymetadata.MappingGroup, 0, len(mapping))
	groups = append(groups, firstGroup)

	lastGroup := firstGroup
	for _, storedKeyIndex := range mapping[1:] {
		if !lastGroup.TryMerge(storedKeyIndex) {
			// Create and append new group
			lastGroup = accountkeymetadata.NewMappingGroup(1, storedKeyIndex, false)
			groups = append(groups, lastGroup)
		}
	}

	return accountkeymetadata.MappingGroups(groups).Encode(), nil
}

func decodeAccountPublicKeyMapping(b []byte) ([]uint32, error) {
	if len(b)%mappingGroupSize != 0 {
		return nil, fmt.Errorf("failed to decode mappings: expect multiple of %d bytes, got %d", mappingGroupSize, len(b))
	}

	mapping := make([]uint32, 0, len(b)/mappingGroupSize)

	for i := 0; i < len(b); i += mappingGroupSize {
		runLength := binary.BigEndian.Uint16(b[i:])
		storedKeyIndex := binary.BigEndian.Uint32(b[i+runLengthSize:])

		if consecutiveBit := (runLength & consecutiveGroupFlagMask) >> 15; consecutiveBit == 1 {
			runLength &= lengthMask

			for i := range runLength {
				mapping = append(mapping, storedKeyIndex+uint32(i))
			}
		} else {
			for range runLength {
				mapping = append(mapping, storedKeyIndex)
			}
		}
	}

	return mapping, nil
}

// Digest list

const digestSize = 8

// encodeDigestList encodes digests into concatenated uint64.
func encodeDigestList(digests []uint64) []byte {
	if len(digests) == 0 {
		return nil
	}
	encodedDigestList := make([]byte, digestSize*len(digests))
	off := 0
	for _, digest := range digests {
		binary.BigEndian.PutUint64(encodedDigestList[off:], digest)
		off += digestSize
	}
	return encodedDigestList
}

func decodeDigestList(b []byte) ([]uint64, error) {
	if len(b)%digestSize != 0 {
		return nil, fmt.Errorf("failed to decode digest list: expect multiple of %d byte, got %d", digestSize, len(b))
	}

	storedDigestCount := len(b) / digestSize

	digests := make([]uint64, 0, storedDigestCount)

	for i := 0; i < len(b); i += digestSize {
		digests = append(digests, binary.BigEndian.Uint64(b[i:]))
	}

	return digests, nil
}

// Public Key Batch Register

const (
	maxEncodedKeySize = math.MaxUint8 // Encoded public key size is ~70 bytes
)

// PublicKeyBatch register contains up to maxBatchPublicKeyCount number of encoded public keys.
// Each public key is encoded as:
// - length prefixed encoded public key
func encodePublicKeysInBatches(encodedPublicKey [][]byte, maxPublicKeyCountInBatch int) ([][]byte, error) {
	// Return early if there is only one encoded public key (first public key).
	// First public key is stored in its own register, not in batch public key register.
	if len(encodedPublicKey) <= 1 {
		return nil, nil
	}

	// Reset first encoded public key to nil during encoding
	// to avoid encoding first account public key in batch public key.

	firstEncodedPublicKey := encodedPublicKey[0]
	defer func() {
		encodedPublicKey[0] = firstEncodedPublicKey
	}()

	encodedPublicKey[0] = nil

	values := make([][]byte, 0, len(encodedPublicKey)/maxPublicKeyCountInBatch+1)

	for i := 0; i < len(encodedPublicKey); {
		batchCount := min(maxPublicKeyCountInBatch, len(encodedPublicKey)-i)

		encodedBatchPublicKey, err := encodeBatchPublicKey(encodedPublicKey[i : i+batchCount])
		if err != nil {
			return nil, err
		}

		values = append(values, encodedBatchPublicKey)

		i += batchCount
	}

	return values, nil
}

func encodeBatchPublicKey(encodedPublicKey [][]byte) ([]byte, error) {

	size := 0
	for _, encoded := range encodedPublicKey {
		if len(encoded) > maxEncodedKeySize {
			return nil, fmt.Errorf("encoded key size is %d bytes, exceeded max size %d", len(encoded), maxEncodedKeySize)
		}
		size += 1 + len(encoded)
	}

	buf := make([]byte, size)
	off := 0
	for _, encoded := range encodedPublicKey {
		buf[off] = byte(len(encoded))
		off++

		n := copy(buf[off:], encoded)
		off += n
	}

	return buf, nil
}

func decodeBatchPublicKey(b []byte) ([][]byte, error) {
	if len(b) == 0 {
		return nil, nil
	}

	encodedPublicKeys := make([][]byte, 0, maxPublicKeyCountInBatch)

	off := 0
	for off < len(b) {
		size := int(b[off])
		off++

		if off+size > len(b) {
			return nil, fmt.Errorf("failed to decode batch public key: off %d + size %d out of bounds %d: %x", off, size, len(b), b)
		}

		encodedPublicKey := b[off : off+size]
		off += size

		encodedPublicKeys = append(encodedPublicKeys, encodedPublicKey)
	}

	if off != len(b) {
		return nil, fmt.Errorf("failed to decode batch public key: trailing data (%d bytes): %x", len(b)-off, b)
	}

	return encodedPublicKeys, nil
}

// Account Status register

const (
	versionMask                                     = 0xf0
	flagMask                                        = 0x0f
	deduplicatedAccountStatusV4VerionAndFlagByte    = 0x41
	nondeduplicatedAccountStatusV4VerionAndFlagByte = 0x40
	accountStatusV4MinimumSize                      = 29 // Same size as account status v3
)

// encodeAccountStatusV4WithPublicKeyMetadata encodes public key metadata section
// in "a.s" register depending on deduplicated flag.
//
// With deduplicated flag, account status is encoded as:
// - account status v3 (29 bytes)
// - length prefixed list of account public key weight and revoked status starting from key index 1
// - startKeyIndex (4 bytes) + length prefixed list of account public key index mappings to stored key index
// - startStoredKeyIndex (4 bytes) + length prefixed list of last N stored key digests
//
// Without deduplicated flag, account status is encoded as:
// - account status v3 (29 bytes)
// - length prefixed list of account public key weight and revoked status starting from key index 1
// - startStoredKeyIndex (4 bytes) + length prefixed list of last N stored key digests
func encodeAccountStatusV4WithPublicKeyMetadata(
	original []byte,
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForDigests uint32,
	keyDigests []uint64,
	startKeyIndexForMappings uint32,
	accountPublicKeyMappings []uint32,
	deduplicated bool,
) ([]byte, error) {

	// Return early if the original account status payload contains any optional fields.
	if len(original) != accountStatusV4MinimumSize {
		return nil, fmt.Errorf("failed to encode account status payload: original payload has %d bytes, expect %d bytes", len(original), accountStatusV4MinimumSize)
	}

	// Encode list of account public key weight and revoked status
	encodedAccountPublicKeyWeightAndRevokedStatus, err := encodeAccountPublicKeyWeightsAndRevokedStatus(weightAndRevokedStatus)
	if err != nil {
		return nil, err
	}

	// Encode list of key digests
	encodedKeyDigests := encodeDigestList(keyDigests)

	// Encode mappings for deduplicated account public keys
	var encodedAccountPublicKeyMapping []byte
	if deduplicated {
		encodedAccountPublicKeyMapping, err = encodeAccountPublicKeyMapping(accountPublicKeyMappings)
		if err != nil {
			return nil, err
		}
	}

	newAccountStatusPayloadSize := len(original) +
		lengthPrefixSize + len(encodedAccountPublicKeyWeightAndRevokedStatus) + // length prefixed account public key weight and revoked status
		4 + // start stored key index for digests
		lengthPrefixSize + len(encodedKeyDigests) // length prefixed digests

	if deduplicated {
		newAccountStatusPayloadSize += 4 + // start key index for mapping
			lengthPrefixSize + len(encodedAccountPublicKeyMapping) // used to retrieve account public key
	}

	buf := make([]byte, newAccountStatusPayloadSize)
	off := 0

	// Append account status v4 version and flag
	if deduplicated {
		buf[0] = deduplicatedAccountStatusV4VerionAndFlagByte
	} else {
		buf[0] = nondeduplicatedAccountStatusV4VerionAndFlagByte
	}
	off++

	// Append original content, except for the flag byte
	n := copy(buf[off:], original[1:])
	off += n

	// Append length prefixed encoded revoked status
	binary.BigEndian.PutUint32(buf[off:], uint32(len(encodedAccountPublicKeyWeightAndRevokedStatus)))
	off += 4

	n = copy(buf[off:], encodedAccountPublicKeyWeightAndRevokedStatus)
	off += n

	if deduplicated {
		// Append start key index for mapping
		binary.BigEndian.PutUint32(buf[off:], startKeyIndexForMappings)
		off += 4

		// Append length prefixed account public key mapping
		binary.BigEndian.PutUint32(buf[off:], uint32(len(encodedAccountPublicKeyMapping)))
		off += 4

		n = copy(buf[off:], encodedAccountPublicKeyMapping)
		off += n
	}

	// Append start key index for digests
	binary.BigEndian.PutUint32(buf[off:], startKeyIndexForDigests)
	off += 4

	// Append length prefixed key digests
	binary.BigEndian.PutUint32(buf[off:], uint32(len(encodedKeyDigests)))
	off += 4

	n = copy(buf[off:], encodedKeyDigests)
	off += n

	return buf[:off], nil
}

func decodeAccountStatusKeyMetadata(b []byte, deduplicated bool) (
	weightAndRevokedStatus []accountPublicKeyWeightAndRevokedStatus,
	startKeyIndexForMapping uint32,
	accountPublicKeyMappings []uint32,
	startKeyIndexForDigests uint32,
	digests []uint64,
	err error,
) {
	// Decode weight and revoked list

	var weightAndRevokedGroupsData []byte
	weightAndRevokedGroupsData, b, err = parseNextLengthPrefixedData(b)
	if err != nil {
		err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
		return
	}

	weightAndRevokedStatus, err = decodeAccountPublicKeyWeightAndRevokedStatusGroups(weightAndRevokedGroupsData)
	if err != nil {
		err = fmt.Errorf("failed to decode weight and revoked status list: %w", err)
		return
	}

	// Decode account public key mapping if deduplication is on

	if deduplicated {
		if len(b) < 4 {
			err = fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start key index for mapping, got %d bytes", len(b))
			return
		}

		startKeyIndexForMapping = binary.BigEndian.Uint32(b)

		b = b[4:]

		var mappingData []byte
		mappingData, b, err = parseNextLengthPrefixedData(b)
		if err != nil {
			err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
			return
		}

		accountPublicKeyMappings, err = decodeAccountPublicKeyMapping(mappingData)
		if err != nil {
			err = fmt.Errorf("failed to decode account public key mappings: %w", err)
			return
		}
	}

	// Decode digests list

	if len(b) < 4 {
		err = fmt.Errorf("failed to decode AccountStatusV4: expect 4 bytes of start stored key index for digests, got %d bytes", len(b))
		return
	}

	startKeyIndexForDigests = binary.BigEndian.Uint32(b)
	b = b[4:]

	var digestsData []byte
	digestsData, b, err = parseNextLengthPrefixedData(b)
	if err != nil {
		err = fmt.Errorf("failed to decode AccountStatusV4: %w", err)
		return
	}

	digests, err = decodeDigestList(digestsData)
	if err != nil {
		err = fmt.Errorf("failed to decode digests: %w", err)
		return
	}

	// Check trailing data

	if len(b) != 0 {
		err = fmt.Errorf("failed to decode AccountStatusV4: got %d extra bytes", len(b))
		return
	}

	return
}

func parseNextLengthPrefixedData(b []byte) (next []byte, rest []byte, err error) {
	if len(b) < lengthPrefixSize {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least 4 bytes, got %d bytes", len(b))
	}

	length := binary.BigEndian.Uint32(b[:lengthPrefixSize])

	if len(b) < lengthPrefixSize+int(length) {
		return nil, nil, fmt.Errorf("failed to decode data: expect at least %d bytes, got %d bytes", lengthPrefixSize+int(length), len(b))
	}

	b = b[lengthPrefixSize:]
	return b[:length], b[length:], nil
}

// Stored Public Key

func encodeStoredPublicKeyFromAccountPublicKey(a flow.AccountPublicKey) ([]byte, error) {
	storedPublicKey := flow.StoredPublicKey{
		PublicKey: a.PublicKey,
		SignAlgo:  a.SignAlgo,
		HashAlgo:  a.HashAlgo,
	}
	return flow.EncodeStoredPublicKey(storedPublicKey)
}
