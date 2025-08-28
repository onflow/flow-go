package migrations

import (
	"encoding/binary"
	"fmt"
	"math"

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

// Account Public Key Index to Stored Public Key Index Mappings

const (
	maxRunLengthInEncodedMappingGroup = 1<<15 - 1
	storedKeyIndexSize                = 4
	mappingGroupSize                  = runLengthSize + storedKeyIndexSize
	consecutiveGroupFlagMask          = 0x8000
	lengthMask                        = 0x7fff
)

type mappingGroup struct {
	runLength      uint32 // runLength is uint32 to prevent overflow
	storedKeyIndex uint32
}

type mappingGroups []mappingGroup

// mappingGroups is encoded using RLE:
// - run length (2 bytes): consecutive group flag in high 1 bit and run length in 15 bits
// - value (4 bytes): stored key index
// NOTE:
//   - If number of elements in a run-length group exceeds maxRunLengthInEncodedMappingGroup,
//     a new group is created with remaining run-length and the same storedKeyIndex.
//   - Consecutive groups are adjoining groups that run-length is 1 and value is increased by 1.
//   - When consecutive group flag is on, run-length is number of consecutive groups, and value is the value of the first group.
func (groups mappingGroups) Encode() []byte {
	if len(groups) == 0 {
		return nil
	}

	buf := make([]byte, 0, len(groups)*(mappingGroupSize))
	off := 0

	for i := 0; i < len(groups); {
		group := groups[i]

		switch group.runLength {
		case 0:
			panic(fmt.Sprintf("run length shouldn't be 0, mapping groups %+v", groups))

		case 1:
			// Handle consecutive groups

			consecutiveGroupCount := uint32(1)
			consecutiveStartStoredKeyIndex := group.storedKeyIndex

			nextGroupIndex := i + 1
			for nextGroupIndex < len(groups) {
				nextGroup := groups[nextGroupIndex]
				if nextGroup.runLength == 1 &&
					nextGroup.storedKeyIndex == consecutiveStartStoredKeyIndex+consecutiveGroupCount {
					consecutiveGroupCount++
					nextGroupIndex++
				} else {
					break
				}
			}

			if consecutiveGroupCount == 1 {
				// Encode regular mapping group
				buf, off = encodeMappingGroup(buf, off, group.runLength, group.storedKeyIndex, false)
			} else {
				// Encode consecutive mapping groups
				buf, off = encodeMappingGroup(buf, off, consecutiveGroupCount, consecutiveStartStoredKeyIndex, true)
			}

			i = nextGroupIndex

		default:
			buf, off = encodeMappingGroup(buf, off, group.runLength, group.storedKeyIndex, false)
			i++
		}
	}

	return buf
}

func encodeMappingGroup(buf []byte, off int, runLength uint32, value uint32, isConsecutiveGroup bool) ([]byte, int) {
	encodedRunLength := uint32(0)

	for encodedRunLength < runLength {
		// NOTE: if number of elements in a group exceeds maxMappingCountInGroup, a new group is created with the same value.

		runLength := min(runLength-encodedRunLength, maxRunLengthInEncodedMappingGroup)

		if cap(buf) >= off+mappingGroupSize {
			buf = buf[:off+mappingGroupSize]
		} else {
			buf = append(buf, make([]byte, mappingGroupSize)...)
		}

		if isConsecutiveGroup {
			binary.BigEndian.PutUint16(buf[off:], uint16(runLength)|consecutiveGroupFlagMask)
		} else {
			binary.BigEndian.PutUint16(buf[off:], uint16(runLength))
		}
		off += runLengthSize

		binary.BigEndian.PutUint32(buf[off:], value)
		off += storedKeyIndexSize

		encodedRunLength += runLength
	}

	return buf, off
}

// encodeAccountPublicKeyMapping encodes keyIndexMappings into concatenated run-length groups.
// Each run-length group is encoded as:
// - length (2 bytes) with max length as 2<<15-1
// - stored key index (4 bytes)
func encodeAccountPublicKeyMapping(mapping []uint32) ([]byte, error) {
	if len(mapping) == 0 {
		return nil, nil
	}

	groups := make(mappingGroups, 0, len(mapping))

	i := 0
	for i < len(mapping) {
		runLength := 1
		value := mapping[i]
		i++

		for i < len(mapping) && mapping[i] == value {
			runLength++
			i++
		}

		groups = append(groups, mappingGroup{runLength: uint32(runLength), storedKeyIndex: value})
	}

	return groups.Encode(), nil
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

// Stored Public Key

func encodeStoredPublicKeyFromAccountPublicKey(a flow.AccountPublicKey) ([]byte, error) {
	storedPublicKey := flow.StoredPublicKey{
		PublicKey: a.PublicKey,
		SignAlgo:  a.SignAlgo,
		HashAlgo:  a.HashAlgo,
	}
	return flow.EncodeStoredPublicKey(storedPublicKey)
}
