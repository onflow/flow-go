package reporters

import (
	"bytes"
	"errors"
	"fmt"
	"math"

	"github.com/fxamacker/cbor/v2"
)

const (
	versionAndFlagSize = 2
	mapExtraDataLength = 3
	storageIDSize      = 16
	digestSize         = 8
	flagIndex          = 1
)

const (
	CBORTagInlineCollisionGroup   = 253
	CBORTagExternalCollisionGroup = 254
)

var (
	encodedInlineCollisionGroupPrefix   = []byte{0xd8, CBORTagInlineCollisionGroup}
	encodedExternalCollisionGroupPrefix = []byte{0xd8, CBORTagExternalCollisionGroup}
)

var decMode = func() cbor.DecMode {
	decMode, err := cbor.DecOptions{
		IntDec:           cbor.IntDecConvertNone,
		MaxArrayElements: math.MaxInt64,
		MaxMapPairs:      math.MaxInt64,
		MaxNestedLevels:  math.MaxInt16,
	}.DecMode()
	if err != nil {
		panic(err)
	}
	return decMode
}()

// These functions are simplified version of decoding
// functions in onflow/atree.  Full functionality requires
// Cadence package which isn't needed here.

// parseSlabMapData returns raw elements bytes.
func parseSlabMapData(data []byte) ([]byte, error) {
	// Check minimum data length
	if len(data) < versionAndFlagSize {
		return nil, errors.New("data is too short for map data slab")
	}

	isRootSlab := isRoot(data[flagIndex])

	// Check flag for extra data
	if isRootSlab {
		// Decode extra data
		var err error
		data, err = skipMapExtraData(data, decMode)
		if err != nil {
			return nil, err
		}
	}

	if len(data) < versionAndFlagSize {
		return nil, errors.New("data is too short for map data slab")
	}

	// Check flag
	flag := data[flagIndex]
	mapType := getSlabMapType(flag)
	if mapType != slabMapData && mapType != slabMapCollisionGroup {
		return nil, fmt.Errorf(
			"data has invalid flag 0x%x, want 0x%x or 0x%x",
			flag,
			maskMapData,
			maskCollisionGroup,
		)
	}

	contentOffset := versionAndFlagSize
	if !isRootSlab {
		// Skip next storage ID for non-root slab
		contentOffset += storageIDSize
	}

	return data[contentOffset:], nil
}

// getCollisionGroupCountFromSlabMapData returns collision level,
// number of collision groups (inline and external collision groups),
// and error.
func getCollisionGroupCountFromSlabMapData(data []byte) (collisionLevel uint, collisionGroupCount uint, err error) {
	elements, err := parseSlabMapData(data)
	if err != nil {
		return 0, 0, err
	}

	collisionLevel, rawElements, err := parseRawElements(elements, decMode)
	if err != nil {
		return 0, 0, err
	}

	for _, rawElem := range rawElements {
		if bytes.HasPrefix(rawElem, encodedInlineCollisionGroupPrefix) ||
			bytes.HasPrefix(rawElem, encodedExternalCollisionGroupPrefix) {
			collisionGroupCount++
		}
	}

	return collisionLevel, collisionGroupCount, nil
}

// getInlineCollisionCountsFromSlabMapData returns collision level, inline collision counts, and error.
func getInlineCollisionCountsFromSlabMapData(data []byte) (collisionLevel uint, inlineCollisionCount []uint, err error) {
	elements, err := parseSlabMapData(data)
	if err != nil {
		return 0, nil, err
	}

	collisionLevel, rawElements, err := parseRawElements(elements, decMode)
	if err != nil {
		return 0, nil, err
	}

	for _, rawElem := range rawElements {
		if bytes.HasPrefix(rawElem, encodedInlineCollisionGroupPrefix) {
			_, collisionElems, err := parseRawElements(rawElem, decMode)
			if err != nil {
				return 0, nil, err
			}
			inlineCollisionCount = append(inlineCollisionCount, uint(len(collisionElems)))
		}
	}

	return collisionLevel, inlineCollisionCount, nil
}

func skipMapExtraData(data []byte, decMode cbor.DecMode) ([]byte, error) {
	// Check data length
	if len(data) < versionAndFlagSize {
		return data, errors.New("data is too short for map extra data")
	}

	// Check flag
	flag := data[1]
	if !isRoot(flag) {
		return data, fmt.Errorf("data has invalid flag 0x%x, want root flag", flag)
	}

	// Decode extra data
	r := bytes.NewReader(data[versionAndFlagSize:])
	dec := decMode.NewDecoder(r)

	var v []any
	err := dec.Decode(&v)
	if err != nil {
		return data, errors.New("failed to decode map extra data")
	}

	if len(v) != mapExtraDataLength {
		return data, fmt.Errorf("map extra data has %d number of elements, want %d", len(v), mapExtraDataLength)
	}

	// Reslice for remaining data
	n := dec.NumBytesRead()
	data = data[versionAndFlagSize+n:]

	return data, nil
}

type elements struct {
	_           struct{} `cbor:",toarray"`
	Level       uint
	DigestBytes []byte
	RawElements []cbor.RawMessage
}

func parseRawElements(data []byte, decMode cbor.DecMode) (uint, []cbor.RawMessage, error) {
	var elems elements
	err := decMode.Unmarshal(data, &elems)
	if err != nil {
		return 0, nil, err
	}

	if len(elems.DigestBytes)%digestSize != 0 {
		return 0, nil, fmt.Errorf("number of digest bytes is not multiple of %d", digestSize)
	}

	digestCount := len(elems.DigestBytes) / digestSize

	if digestCount != len(elems.RawElements) {
		return 0, nil, fmt.Errorf("found %d digests and %d elements", digestCount, len(elems.RawElements))
	}

	return elems.Level, elems.RawElements, nil
}

// The remaining code is a subset of onflow/atree/flag.go.
// These functions are not exported by onflow/atree because
// they are implementation details.  They are copied here
// to parse atree slabs.

type slabType int

const (
	slabTypeUndefined slabType = iota
	slabArray
	slabMap
	slabStorable
)

type slabArrayType int

const (
	slabArrayUndefined slabArrayType = iota
	slabArrayData
	slabArrayMeta
	slabLargeImmutableArray
	slabBasicArray
)

type slabMapType int

const (
	slabMapUndefined slabMapType = iota
	slabMapData
	slabMapMeta
	slabMapLargeEntry
	slabMapCollisionGroup
)

const (
	// Slab flags: 3 high bits
	maskSlabRoot byte = 0b100_00000
	// maskSlabHasPointers byte = 0b010_00000
	// maskSlabAnySize     byte = 0b001_00000

	// Array flags: 3 low bits (4th and 5th bits are 0)
	// maskArrayData byte = 0b000_00000
	// maskArrayMeta byte = 0b000_00001
	// maskLargeImmutableArray byte = 0b000_00010 // not used for now
	// maskBasicArray byte = 0b000_00011 // used for benchmarking

	// Map flags: 3 low bits (4th bit is 0, 5th bit is 1)
	maskMapData byte = 0b000_01000
	// maskMapMeta byte = 0b000_01001
	// maskLargeMapEntry  byte = 0b000_01010 // not used for now
	maskCollisionGroup byte = 0b000_01011

	// Storable flags: 3 low bits (4th bit is 1, 5th bit is 1)
	// maskStorable byte = 0b000_11111
)

func isRoot(f byte) bool {
	return f&maskSlabRoot > 0
}

func getSlabType(f byte) slabType {
	// Extract 4th and 5th bits for slab type.
	dataType := (f & byte(0b000_11000)) >> 3
	switch dataType {
	case 0:
		// 4th and 5th bits are 0.
		return slabArray
	case 1:
		// 4th bit is 0 and 5th bit is 1.
		return slabMap
	case 3:
		// 4th and 5th bit are 1.
		return slabStorable
	default:
		return slabTypeUndefined
	}
}

func getSlabArrayType(f byte) slabArrayType {
	if getSlabType(f) != slabArray {
		return slabArrayUndefined
	}

	// Extract 3 low bits for slab array type.
	dataType := (f & byte(0b000_00111))
	switch dataType {
	case 0:
		return slabArrayData
	case 1:
		return slabArrayMeta
	case 2:
		return slabLargeImmutableArray
	case 3:
		return slabBasicArray
	default:
		return slabArrayUndefined
	}
}

func getSlabMapType(f byte) slabMapType {
	if getSlabType(f) != slabMap {
		return slabMapUndefined
	}

	// Extract 3 low bits for slab map type.
	dataType := (f & byte(0b000_00111))
	switch dataType {
	case 0:
		return slabMapData
	case 1:
		return slabMapMeta
	case 2:
		return slabMapLargeEntry
	case 3:
		return slabMapCollisionGroup
	default:
		return slabMapUndefined
	}
}
