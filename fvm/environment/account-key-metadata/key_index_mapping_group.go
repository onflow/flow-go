package accountkeymetadata

import (
	"encoding/binary"

	"github.com/onflow/flow-go/fvm/errors"
)

// Account Public Key Index to Stored Public Key Index Mappings

// Key index mapping is encoded using RLE:
// - run length (2 bytes): consecutive group flag in high 1 bit and run length in 15 bits
// - value (4 bytes): stored key index
// NOTE:
//   - If number of elements in a run-length group exceeds maxRunLengthInMappingGroup,
//     a new group is created with remaining run-length and the same storedKeyIndex.
//   - Consecutive groups are adjoining groups that run-length is 1 and value is increased by 1.
//   - When consecutive group flag is on, run-length is number of consecutive groups, and value is the value of the first group.

// We encode stored key indexes using a new variant of RLE designed to efficiently store
// long runs of unique integers that increment by 1.
// For example, if an account has 1000 keys and the first 2 keys are the same, followed
// by 998 unique keys (e.g., {key0, key0, key1, key2, ..., key998}).
// Using regular RLE would bloat the storage size by storing 999 mapping groups with run length 1.
// By comparison, our optimized encoding would only store 2 mapping groups.
// We support a maximum run length of 32767 because we use the low 15 bits of uint16
// to store the run length (the high bit is used as a flag).

const (
	maxRunLengthInMappingGroup = 1<<15 - 1

	storedKeyIndexSize = 4
	mappingGroupSize   = runLengthSize + storedKeyIndexSize

	consecutiveGroupFlagMask = 0x8000
	lengthMask               = 0x7fff
)

// getStoredKeyIndexFromMappings returns stored key index of the given key index from encoded data.
// Received b is expected to only contain encoded mappings.
func getStoredKeyIndexFromMappings(b []byte, keyIndex uint32) (uint32, error) {
	remainingKeyIndex := keyIndex

	if len(b)%mappingGroupSize != 0 {
		return 0,
			errors.NewKeyMetadataUnexpectedLengthError(
				"failed to get stored key index from mapping",
				mappingGroupSize,
				len(b),
			)
	}

	for off := 0; off < len(b); off += mappingGroupSize {
		isConsecutiveGroup, runLength := parseMappingRunLength(b, off)

		if remainingKeyIndex < uint32(runLength) {
			storedKeyIndex := parseMappingStoredKeyIndex(b, off+runLengthSize)

			if isConsecutiveGroup {
				return storedKeyIndex + remainingKeyIndex, nil
			}

			return storedKeyIndex, nil
		}

		remainingKeyIndex -= uint32(runLength)
	}

	return 0, errors.NewKeyMetadataNotFoundError("failed to query stored key index from mapping", keyIndex)
}

// appendStoredKeyIndexToMappings appends the given storedKeyIndex to the given encoded mappings (b).
// NOTE: b can be modified by this function.
func appendStoredKeyIndexToMappings(b []byte, storedKeyIndex uint32) (_ []byte, _ error) {
	if len(b) == 0 {
		return encodeKeyIndexToStoredKeyIndexMapping(false, 1, storedKeyIndex), nil
	}

	if len(b)%mappingGroupSize != 0 {
		return nil,
			errors.NewKeyMetadataUnexpectedLengthError(
				"failed to append stored key mapping",
				mappingGroupSize,
				len(b),
			)
	}

	// Merge to last group
	lastGroupOff := len(b) - mappingGroupSize
	lastGroup := parseMappingGroup(b, lastGroupOff)

	if lastGroup.TryMerge(storedKeyIndex) {
		// Overwrite the last group in the given b since the
		// given storedKeyIndex is merged into the last group.
		b = append(b[:lastGroupOff], lastGroup.Encode()...)
		return b, nil
	}

	// Append new group
	b = append(b, encodeKeyIndexToStoredKeyIndexMapping(false, 1, storedKeyIndex)...)
	return b, nil
}

// Utils

type MappingGroup struct {
	runLength        uint16
	storedKeyIndex   uint32
	consecutiveGroup bool
}

func NewMappingGroup(runLength uint16, storedKeyIndex uint32, consecutive bool) *MappingGroup {
	return &MappingGroup{
		runLength:        runLength,
		storedKeyIndex:   storedKeyIndex,
		consecutiveGroup: consecutive,
	}
}

// TryMerge returns true if the given storedKeyIndex is merged into g.
// The given storedKeyIndex is merged into regular group g if
// g's runLength is less than maxRunLengthInMappingGroup and
// either g's storedKeyIndex is the same as the given storedKeyIndex or
// g's runLength is 1 and g's storedKeyIndex + 1 is the same as the
// given storedKeyIndex.
// The given storedKeyIndex is merged into consecutive group g if
// g's runLength is less than maxRunLengthInMappingGroup and
// g's storedKeyIndex + g's runLength the same as the given storedKeyIndex.
func (g *MappingGroup) TryMerge(storedKeyIndex uint32) bool {
	if g.runLength == maxRunLengthInMappingGroup {
		// Can't be merged because run length limit is reached.
		return false
	}

	if g.consecutiveGroup {
		if g.storedKeyIndex+uint32(g.runLength) == storedKeyIndex {
			// Merge into consecutive group
			g.runLength++
			return true
		}
		// Can't be merged because new stored key index isn't consecutive.
		return false
	}

	if g.storedKeyIndex == storedKeyIndex {
		// Merge into regular group
		g.runLength++
		return true
	}

	if g.runLength == 1 && g.storedKeyIndex+1 == storedKeyIndex {
		// Convert the last group from a regular group to a consecutive group,
		// and merge the given storedKeyIndex into it.
		g.consecutiveGroup = true
		g.runLength++
		return true
	}

	return false
}

func (g *MappingGroup) Encode() []byte {
	return encodeKeyIndexToStoredKeyIndexMapping(g.consecutiveGroup, g.runLength, g.storedKeyIndex)
}

type MappingGroups []*MappingGroup

func (groups MappingGroups) Encode() []byte {
	if len(groups) == 0 {
		return nil
	}

	buf := make([]byte, 0, len(groups)*(mappingGroupSize))
	for _, group := range groups {
		buf = append(buf, group.Encode()...)
	}
	return buf
}

func encodeKeyIndexToStoredKeyIndexMapping(isConsecutiveGroup bool, runLength uint16, storedKeyIndex uint32) []byte {
	var b [mappingGroupSize]byte

	if isConsecutiveGroup {
		runLength |= consecutiveGroupFlagMask
	}
	// Set runlength
	binary.BigEndian.PutUint16(b[:], runLength)

	// Set value
	binary.BigEndian.PutUint32(b[runLengthSize:], storedKeyIndex)

	return b[:]
}

func parseMappingGroup(b []byte, off int) *MappingGroup {
	_ = b[off+mappingGroupSize-1] // bounds check
	isConsecutiveGroup, runLength := parseMappingRunLength(b, off)
	storedKeyIndex := binary.BigEndian.Uint32(b[off+runLengthSize:])
	return &MappingGroup{
		runLength:        runLength,
		storedKeyIndex:   storedKeyIndex,
		consecutiveGroup: isConsecutiveGroup,
	}
}

func parseMappingRunLength(b []byte, off int) (isConsecutiveGroup bool, runLength uint16) {
	_ = b[off+1] // bounds check
	runLength = binary.BigEndian.Uint16(b[off : off+runLengthSize])
	isConsecutiveGroup = runLength&consecutiveGroupFlagMask > 0
	runLength &= lengthMask
	return
}

func parseMappingStoredKeyIndex(b []byte, off int) uint32 {
	_ = b[off+3] // bounds check
	return binary.BigEndian.Uint32(b[off : off+storedKeyIndexSize])
}

// Utility functions for tests and validation

// DecodeMappings decodes raw bytes of account public key mappings in account key metadata.
func DecodeMappings(b []byte) ([]uint32, error) {
	if len(b)%mappingGroupSize != 0 {
		return nil, errors.NewKeyMetadataUnexpectedLengthError(
			"failed to decode key mappings",
			mappingGroupSize,
			len(b),
		)
	}

	mapping := make([]uint32, 0, len(b)/mappingGroupSize)

	for i := 0; i < len(b); i += mappingGroupSize {

		isConsecutiveGroup, runLength := parseMappingRunLength(b, i)
		storedKeyIndex := binary.BigEndian.Uint32(b[i+runLengthSize:])

		if isConsecutiveGroup {
			for index := range runLength {
				mapping = append(mapping, storedKeyIndex+uint32(index))
			}
		} else {
			for range runLength {
				mapping = append(mapping, storedKeyIndex)
			}
		}
	}

	return mapping, nil
}
