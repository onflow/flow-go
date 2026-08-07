package pebble

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

// latestHeightKey is a special case of a lookupKey
// with keyLatestBlockHeight as key, no owner and a placeholder height of 0.
// This is to ensure SeekPrefixGE in pebble does not break
var latestHeightKey = binary.BigEndian.AppendUint64(
	[]byte{codeLatestBlockHeight, byte('/'), byte('/')}, placeHolderHeight)

// firstHeightKey is a special case of a lookupKey
// with keyFirstBlockHeight as key, no owner and a placeholder height of 0.
// This is to ensure SeekPrefixGE in pebble does not break
var firstHeightKey = binary.BigEndian.AppendUint64(
	[]byte{codeFirstBlockHeight, byte('/'), byte('/')}, placeHolderHeight)

// lookupKey is the encoded format of the storage key for looking up register value
type lookupKey struct {
	encoded []byte
}

// newLookupKey takes a height and registerID, returns the key for storing the register value in storage
//
// Lookup keys are encoded as follows:
// [codeRegister(1)] [owner] '/' [key] '/' [height(8)]
// owner and key are variable length fields.
//
// Note: owner must be either empty (global registers) or exactly flow.AddressLength bytes
// (account registers). lookupKeyToRegisterID relies on this invariant to decode the owner,
// since both owner and key may contain the '/' separator byte.
func newLookupKey(height uint64, reg flow.RegisterID) *lookupKey {
	key := lookupKey{
		// 1 byte gaps for db prefix and '/' separators
		encoded: make([]byte, 0, MinLookupKeyLen+len(reg.Owner)+len(reg.Key)),
	}

	// append DB prefix
	key.encoded = append(key.encoded, codeRegister)

	// The lookup key used to find most recent value for a register.
	//
	// The "<owner>/<key>" part is the register key, which is used as a prefix to filter and iterate
	// through updated values at different heights, and find the most recent updated value at or below
	// a certain height.
	key.encoded = append(key.encoded, []byte(reg.Owner)...)
	key.encoded = append(key.encoded, '/')
	key.encoded = append(key.encoded, []byte(reg.Key)...)
	key.encoded = append(key.encoded, '/')

	// Encode the height getting it to 1s compliment (all bits flipped) and big-endian byte order.
	//
	// Registers are a sparse dataset stored with a single entry per update. To find the value at a particular
	// height, we need to do a scan across the entries to find the highest height that is less than or equal
	// to the target height.
	//
	// Pebble does not support reverse iteration, so we use the height's one's complement to effectively
	// reverse sort on the height. This allows us to use a bitwise forward scan for the next most recent
	// entry.
	onesCompliment := ^height
	key.encoded = binary.BigEndian.AppendUint64(key.encoded, onesCompliment)

	return &key
}

// lookupKeyToRegisterID takes a lookup key and decode it into height and RegisterID
func lookupKeyToRegisterID(lookupKey []byte) (uint64, flow.RegisterID, error) {
	if len(lookupKey) < MinLookupKeyLen {
		return 0, flow.RegisterID{},
			fmt.Errorf("invalid lookup key format: expected >= %d bytes, got %d bytes", MinLookupKeyLen, len(lookupKey))
	}

	// 1. Check and exclude db prefix
	prefix := lookupKey[0]
	if prefix != codeRegister {
		return 0, flow.RegisterID{}, fmt.Errorf("invalid lookup key format: incorrect prefix %d for register lookup key, expected %d", prefix, codeRegister)
	}
	lookupKey = lookupKey[1:]

	// 2. Get the height from the end of the key, and remove the trailing separator
	//
	// The height is always exactly HeightSuffixLen bytes at the end of the key. The separator
	// before the height is therefore always at len(lookupKey) - HeightSuffixLen - 1. We compute
	// it directly rather than searching for the last '/', because the one's complement height
	// encoding can contain 0x2F ('/') bytes, which would cause a backward search to find the
	// wrong separator.
	heightPos := len(lookupKey) - registers.HeightSuffixLen

	if lookupKey[heightPos-1] != '/' {
		return 0, flow.RegisterID{},
			fmt.Errorf("invalid lookup key format: expected '/' separator at position %d, got %x", heightPos-1, lookupKey[heightPos-1])
	}

	heightBytes := lookupKey[heightPos:]

	oneCompliment := binary.BigEndian.Uint64(heightBytes)
	height := ^oneCompliment

	lookupKey = lookupKey[:heightPos-1] // remove the trailing separator and height

	// 3. Get the owner and key
	//
	// What remains is [owner] '/' [key]. We cannot search for the separator: the owner is raw
	// address bytes and may itself contain 0x2F ('/'). Instead, decode structurally: the owner
	// is either exactly flow.AddressLength bytes (account registers) or empty (global registers).
	//
	// The account case is checked first so that addresses starting with 0x2F are not misparsed
	// as global registers. This is unambiguous in practice: no global register key contains
	// '/' at index flow.AddressLength-1.
	var owner, key string
	switch {
	case len(lookupKey) > flow.AddressLength && lookupKey[flow.AddressLength] == '/':
		owner = string(lookupKey[:flow.AddressLength])
		key = string(lookupKey[flow.AddressLength+1:])
	case lookupKey[0] == '/':
		// global registers have an empty owner
		owner = ""
		key = string(lookupKey[1:])
	default:
		// only the two cases above are valid; anything else is a malformed key
		return 0, flow.RegisterID{},
			fmt.Errorf("invalid lookup key format: cannot find owner/key separator")
	}

	return height, flow.RegisterID{Owner: owner, Key: key}, nil
}

// Bytes returns the encoded lookup key.
func (h lookupKey) Bytes() []byte {
	return h.encoded
}

// String returns the encoded lookup key as a string.
func (h lookupKey) String() string {
	return string(h.encoded)
}

// encodedUint64 encodes uint64 for storing as a pebble payload
func encodedUint64(height uint64) []byte {
	payload := make([]byte, 0, 8)
	return binary.BigEndian.AppendUint64(payload, height)
}
