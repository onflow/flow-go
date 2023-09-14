package pebble

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/registers"
)

// lookupKey is the encoded format of the storage key for looking up register value
type lookupKey struct {
	encoded []byte
}

func latestHeightKey() []byte {
	return []byte{keyLatestBlockHeight}
}

func firstHeightKey() []byte {
	return []byte{keyFirstBlockHeight}
}

// newLookupKey takes a height and registerID, returns the key for storing the register value in storage
func newLookupKey(height uint64, reg flow.RegisterID) *lookupKey {
	key := lookupKey{
		encoded: make([]byte, 0, 1+len(reg.Owner)+1+len(reg.Key)+1+registers.HeightSuffixLen),
	}

	// append DB prefix
	key.encoded = append(key.encoded, codeRegisterValue)

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
	const minLookupKeyLen = 3 + registers.HeightSuffixLen
	if len(lookupKey) < minLookupKeyLen {
		return 0, flow.RegisterID{}, fmt.Errorf("invalid lookup key format: expected >= %d bytes, got %d bytes",
			minLookupKeyLen, len(lookupKey))
	}

	// exclude db prefix
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
