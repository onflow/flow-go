package pebble

import (
	"bytes"
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
		return 0, flow.RegisterID{}, fmt.Errorf("invalid lookup key format: expected >= %d bytes, got %d bytes",
			MinLookupKeyLen, len(lookupKey))
	}

	// check and exclude db prefix
	prefix := lookupKey[0]
	if prefix != codeRegister {
		return 0, flow.RegisterID{}, fmt.Errorf("incorrect prefix %d for register lookup key, expected %d",
			prefix, codeRegister)
	}
	lookupKey = lookupKey[1:]

	// Find the first slash to split the lookup key and decode the owner.
	firstSlash := bytes.IndexByte(lookupKey, '/')
	if firstSlash == -1 {
		return 0, flow.RegisterID{}, fmt.Errorf("invalid lookup key format: cannot find first slash")
	}

	owner := string(lookupKey[:firstSlash])

	// Find the last slash to split encoded height.
	lastSlashPos := bytes.LastIndexByte(lookupKey, '/')
	if lastSlashPos == firstSlash {
		return 0, flow.RegisterID{}, fmt.Errorf("invalid lookup key format: expected 2 separators, got 1 separator")
	}
	encodedHeightPos := lastSlashPos + 1
	if len(lookupKey)-encodedHeightPos != registers.HeightSuffixLen {
		return 0, flow.RegisterID{},
			fmt.Errorf("invalid lookup key format: expected %d bytes of encoded height, got %d bytes",
				registers.HeightSuffixLen, len(lookupKey)-encodedHeightPos)
	}

	// Decode height.
	heightBytes := lookupKey[encodedHeightPos:]

	oneCompliment := binary.BigEndian.Uint64(heightBytes)
	height := ^oneCompliment

	// Decode the remaining bytes into the key.
	keyBytes := lookupKey[firstSlash+1 : lastSlashPos]
	key := string(keyBytes)

	regID := flow.RegisterID{Owner: owner, Key: key}

	return height, regID, nil
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
