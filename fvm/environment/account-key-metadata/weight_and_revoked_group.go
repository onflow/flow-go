package accountkeymetadata

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/onflow/flow-go/fvm/errors"
)

// Account public key weight and revoked status is encoded using RLE:
// - run length (2 bytes)
// - value (2 bytes): revoked status is the high bit and weight is the remaining 15 bits.
// NOTE: if number of elements in a run-length group exceeds maxRunLengthInWeightAndRevokedStatusGroup,
// a new group is created with remaining run-length and the same weight and revoked status.

const (
	maxRunLengthInWeightAndRevokedStatusGroup = math.MaxUint16

	weightAndRevokedStatusSize      = 2
	weightAndRevokedStatusIndex     = runLengthSize
	weightAndRevokedStatusGroupSize = runLengthSize + weightAndRevokedStatusSize

	revokedMask = 0x8000
	weightMask  = 0x7fff
)

// getWeightAndRevokedStatus returns weight and revoked status of the given keyIndex from encoded data.
// Received b is expected to only contain encoded weight and revoked status.
func getWeightAndRevokedStatus(b []byte, keyIndex uint32) (bool, uint16, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return false, 0,
			errors.NewKeyMetadataUnexpectedLengthError(
				"failed to get weight and revoked status",
				weightAndRevokedStatusGroupSize,
				len(b),
			)
	}

	remainingKeyIndex := keyIndex

	for off := 0; off < len(b); off += weightAndRevokedStatusGroupSize {
		runLength := parseRunLength(b, off)

		if remainingKeyIndex < uint32(runLength) {
			revoked, weight := parseWeightAndRevokedStatus(b, off+runLengthSize)
			return revoked, weight, nil
		}

		remainingKeyIndex -= uint32(runLength)
	}

	return false, 0, errors.NewKeyMetadataNotFoundError("failed to query weight and revoked status", keyIndex)
}

// appendWeightAndRevokedStatus appends given weight and revoked status to the given data.
// New weight and revoked status can be appended in a new run-length group, or included in
// last group by incrementing last group's run-length.
// NOTE: this function can modify the given data.
func appendWeightAndRevokedStatus(b []byte, revoked bool, weight uint16) ([]byte, error) {
	if len(b) == 0 {
		return encodeWeightAndRevokedStatusGroup(1, revoked, weight), nil
	}

	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil,
			errors.NewKeyMetadataUnexpectedLengthError(
				"failed to append weight and revoked status",
				weightAndRevokedStatusGroupSize,
				len(b),
			)
	}

	// Merge to last groupo
	lastGroupOff := len(b) - weightAndRevokedStatusGroupSize
	lastGroup := parseWeightAndRevokedStatusGroup(b, lastGroupOff)

	if lastGroup.tryMerge(revoked, weight) {
		b = append(b[:lastGroupOff], lastGroup.encode()...)
		return b, nil
	}

	b = append(b, encodeWeightAndRevokedStatusGroup(1, revoked, weight)...)
	return b, nil
}

// setRevokedStatus sets revoked status for the given index in the given data.
// NOTE: this function can modify the given data.
func setRevokedStatus(b []byte, index uint32) ([]byte, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil,
			errors.NewKeyMetadataUnexpectedLengthError(
				"failed to set revoked status",
				weightAndRevokedStatusGroupSize,
				len(b),
			)
	}

	foundGroup := false
	curOff := 0
	for ; curOff < len(b); curOff += weightAndRevokedStatusGroupSize {
		runLength := parseRunLength(b, curOff)

		// When this loop exits, index is guaranteed to be less than math.MaxUint16 (65535)
		// because runlength is uint16.
		if index < uint32(runLength) {
			foundGroup = true
			break
		}

		index -= uint32(runLength)
	}

	if !foundGroup {
		return nil, errors.NewKeyMetadataNotFoundError("failed to set revoked status", index)
	}

	curGroup := parseWeightAndRevokedStatusGroup(b, curOff)

	if curGroup.revoked {
		// Revoked status is already true
		return b, nil
	}

	isFirstElementInGroup := index == 0
	isLastElementInGroup := index+1 == uint32(curGroup.runLength)

	// Set revoked status by splitting current group into multiple groups
	newGroups := curGroup.setRevoke(uint16(index))

	// Return early if there is only one group.
	if len(b) == weightAndRevokedStatusGroupSize {
		encodedNewGroups := encodeWeightAndRevokedStatusGroups(newGroups)
		return encodedNewGroups, nil
	}

	startOff := curOff
	endOff := curOff + weightAndRevokedStatusGroupSize

	// Try to merge with previous group
	if isFirstElementInGroup && curOff > 0 {
		prevGroupOff := curOff - weightAndRevokedStatusGroupSize
		prevGroup := parseWeightAndRevokedStatusGroup(b, prevGroupOff)

		firstNewGroup := newGroups[0]
		if merged, modifiedFirstNewGroup := prevGroup.tryMergeGroup(firstNewGroup); merged {
			startOff = prevGroupOff

			if modifiedFirstNewGroup == nil {
				// First new group is completed merged with previous group,
				// so replace first new group with previous group to be re-encoded.
				newGroups[0] = prevGroup
			} else {
				// First new group is partially merged with previous group,
				// so include both previous group and modified first new group to be re-encoded.
				newGroups[0] = modifiedFirstNewGroup
				newGroups = slices.Insert(newGroups, 0, prevGroup)
			}
		}
	}

	// Try to merge with next group
	if isLastElementInGroup {
		nextGroupOff := curOff + weightAndRevokedStatusGroupSize
		if nextGroupOff < len(b) {
			nextGroup := parseWeightAndRevokedStatusGroup(b, nextGroupOff)

			lastNewGroup := newGroups[len(newGroups)-1]
			if merged, modifiedNextGroup := lastNewGroup.tryMergeGroup(nextGroup); merged {
				endOff = nextGroupOff + weightAndRevokedStatusGroupSize

				if modifiedNextGroup != nil {
					// Next group is partially merged with new groups,
					// so include modified next group in new groups to be re-encoded.
					newGroups = append(newGroups, modifiedNextGroup)
				}
			}
		}
	}

	encodedNewGroups := encodeWeightAndRevokedStatusGroups(newGroups)

	if startOff == 0 && endOff == len(b) {
		return encodedNewGroups, nil
	}

	newSize := len(b) - (endOff - startOff) + len(encodedNewGroups)
	newBuffer := make([]byte, 0, newSize)
	newBuffer = append(newBuffer, b[:startOff]...)
	newBuffer = append(newBuffer, encodedNewGroups...)
	newBuffer = append(newBuffer, b[endOff:]...)

	return newBuffer, nil
}

// Utils

type weightAndRevokedStatusGroup struct {
	runLength uint16
	weight    uint16
	revoked   bool
}

func newWeightAndRevokedStatusGroup(runLength uint16, weight uint16, revoked bool) *weightAndRevokedStatusGroup {
	return &weightAndRevokedStatusGroup{
		runLength: runLength,
		weight:    weight,
		revoked:   revoked,
	}
}

func (g *weightAndRevokedStatusGroup) tryMerge(revoked bool, weight uint16) bool {
	if g.revoked != revoked || g.weight != weight || g.runLength == maxRunLengthInWeightAndRevokedStatusGroup {
		return false
	}
	g.runLength++
	return true
}

// tryMergeGroup returns true and nil if next group is completely merged into g.
// It returns true and modified next group if next group is partially merged into g.
// It return false and unmodified next group if merge failed.
// NOTE: next can be modified.
func (g *weightAndRevokedStatusGroup) tryMergeGroup(next *weightAndRevokedStatusGroup) (bool, *weightAndRevokedStatusGroup) {
	// Current group reached run length limit
	if g.runLength == maxRunLengthInWeightAndRevokedStatusGroup {
		return false, next
	}

	// Groups have different values.
	if g.revoked != next.revoked || g.weight != next.weight {
		return false, next
	}

	totalRunLength := uint32(g.runLength) + uint32(next.runLength)

	// Merge second group into first group
	if totalRunLength <= maxRunLengthInWeightAndRevokedStatusGroup {
		g.runLength += next.runLength
		return true, nil
	}

	// Rebalance groups
	g.runLength = maxRunLengthInWeightAndRevokedStatusGroup
	next.runLength = uint16(totalRunLength - maxRunLengthInWeightAndRevokedStatusGroup)
	return true, next
}

func (g *weightAndRevokedStatusGroup) encode() []byte {
	return encodeWeightAndRevokedStatusGroup(g.runLength, g.revoked, g.weight)
}

func (g *weightAndRevokedStatusGroup) setRevoke(index uint16) []*weightAndRevokedStatusGroup {
	if g.runLength == 1 {
		return []*weightAndRevokedStatusGroup{newWeightAndRevokedStatusGroup(1, g.weight, true)}
	}

	groups := make([]*weightAndRevokedStatusGroup, 0, 3)

	// Create group before revoked index
	if index > 0 {
		groups = append(groups, newWeightAndRevokedStatusGroup(index, g.weight, g.revoked))
	}

	// Create group for the revoked index
	groups = append(groups, newWeightAndRevokedStatusGroup(1, g.weight, true))

	// Create group after revoked index
	if index+1 < g.runLength {
		groups = append(groups, newWeightAndRevokedStatusGroup(g.runLength-index-1, g.weight, g.revoked))
	}

	return groups
}

func encodeWeightAndRevokedStatusGroups(weightAndRevoked []*weightAndRevokedStatusGroup) []byte {
	buf := make([]byte, 0, len(weightAndRevoked)*(weightAndRevokedStatusGroupSize))

	for _, g := range weightAndRevoked {
		b := g.encode()
		buf = append(buf, b...)
	}

	return buf
}

func encodeWeightAndRevokedStatusGroup(runLength uint16, revoked bool, weight uint16) []byte {
	var b [weightAndRevokedStatusGroupSize]byte

	// Set runlength
	binary.BigEndian.PutUint16(b[:], runLength)

	// Set value
	value := weight
	if revoked {
		value |= revokedMask
	}
	binary.BigEndian.PutUint16(b[weightAndRevokedStatusIndex:], value)

	return b[:]
}

func parseWeightAndRevokedStatusGroup(b []byte, off int) *weightAndRevokedStatusGroup {
	_ = b[off+weightAndRevokedStatusGroupSize-1] // bounds check
	runLength := parseRunLength(b, off)
	revoked, weight := parseWeightAndRevokedStatus(b, off+runLengthSize)
	return newWeightAndRevokedStatusGroup(runLength, weight, revoked)
}

func parseRunLength(b []byte, off int) uint16 {
	_ = b[off+1] // bounds check
	return binary.BigEndian.Uint16(b[off:])
}

func parseWeightAndRevokedStatus(b []byte, off int) (revoked bool, weight uint16) {
	_ = b[off+1] // bounds check
	weightAndRevoked := binary.BigEndian.Uint16(b[off:])
	weight = weightAndRevoked & weightMask
	revoked = (weightAndRevoked & revokedMask) > 0
	return revoked, weight
}

// Utility functions for tests and validation

// WeightAndRevokedStatus represents weight and revoked status of an account public key.
type WeightAndRevokedStatus struct {
	Weight  uint16
	Revoked bool
}

// DecodeWeightAndRevokedStatuses decodes raw bytes of weight and revoked statuses in account key metadata.
func DecodeWeightAndRevokedStatuses(b []byte) ([]WeightAndRevokedStatus, error) {
	if len(b)%weightAndRevokedStatusGroupSize != 0 {
		return nil, errors.NewKeyMetadataUnexpectedLengthError(
			"failed to decode weight and revoked status",
			weightAndRevokedStatusGroupSize,
			len(b),
		)
	}

	statuses := make([]WeightAndRevokedStatus, 0, len(b)/weightAndRevokedStatusGroupSize)

	for i := 0; i < len(b); i += weightAndRevokedStatusGroupSize {
		runLength := parseRunLength(b, i)
		revoked, weight := parseWeightAndRevokedStatus(b, i+runLengthSize)

		status := WeightAndRevokedStatus{
			Weight:  weight,
			Revoked: revoked,
		}

		for range runLength {
			statuses = append(statuses, status)
		}
	}

	return statuses, nil
}
