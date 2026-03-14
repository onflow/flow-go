package pebble

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// Test_lookupKey_Bytes tests the lookup key encoding format, including the exact byte layout and
// a roundtrip decode.
func Test_lookupKey_Bytes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		height        uint64
		owner         string
		key           string
		expectedBytes []byte
	}{
		{
			name:   "typical register at height 777",
			height: 777,
			owner:  "owner",
			key:    "key",
			// [codeRegister] + "owner/key/" + ^777 as big-endian uint64
			expectedBytes: append([]byte{codeRegister}, []byte("owner/key/\xff\xff\xff\xff\xff\xff\xfc\xf6")...),
		},
		{
			name:   "height 0 encodes as all 0xff",
			height: 0,
			owner:  "a",
			key:    "b",
			// ^0 = MaxUint64 = 0xFFFFFFFFFFFFFFFF
			expectedBytes: append([]byte{codeRegister}, []byte("a/b/\xff\xff\xff\xff\xff\xff\xff\xff")...),
		},
		{
			name:   "max height encodes as all 0x00",
			height: math.MaxUint64,
			owner:  "a",
			key:    "b",
			// ^MaxUint64 = 0
			expectedBytes: append([]byte{codeRegister}, []byte("a/b/\x00\x00\x00\x00\x00\x00\x00\x00")...),
		},
		{
			name:   "empty owner and key",
			height: 777,
			owner:  "",
			key:    "",
			// [codeRegister] + "//" + ^777 as big-endian uint64
			expectedBytes: append([]byte{codeRegister}, []byte("//\xff\xff\xff\xff\xff\xff\xfc\xf6")...),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg := flow.RegisterID{Owner: c.owner, Key: c.key}
			encoded := newLookupKey(c.height, reg).Bytes()

			// prefix byte
			require.Equal(t, byte(codeRegister), encoded[0])

			// height is stored as one's complement (bits flipped) to enable forward iteration
			// in reverse height order: MaxUint64 - storedValue == originalHeight
			storedHeight := binary.BigEndian.Uint64(encoded[len(encoded)-8:])
			require.Equal(t, math.MaxUint64-storedHeight, c.height)

			// full encoding
			require.Equal(t, c.expectedBytes, encoded)

			// roundtrip
			decodedHeight, decodedReg, err := lookupKeyToRegisterID(encoded)
			require.NoError(t, err)
			require.Equal(t, c.height, decodedHeight)
			require.Equal(t, c.owner, decodedReg.Owner)
			require.Equal(t, c.key, decodedReg.Key)
		})
	}
}

// Test_decodeKey_roundtrip tests that encoding then decoding a lookup key reproduces the original
// values, including edge cases where the height encoding contains the separator byte 0x2F ('/').
func Test_decodeKey_roundtrip(t *testing.T) {
	// heightWith0x2FInEncoding is a height whose one's complement encoding contains 0x2F ('/').
	// ^208 = 0xFFFFFFFFFFFFFF2F, so the last byte of the height encoding is '/'.
	// This was previously misinterpreted as the separator between key and height.
	const heightWith0x2FInEncoding = uint64(208)

	cases := []struct {
		name   string
		height uint64
		owner  string
		key    string
	}{
		{name: "key with slash", height: 10, owner: "owneraddress", key: "public/storage/hasslash-in-key"},
		{name: "empty key", height: 10, owner: "owneraddress", key: ""},
		{name: "empty owner", height: 10, owner: "", key: "somekey"},
		{name: "empty owner and key", height: 10, owner: "", key: ""},
		// Heights whose one's complement encoding contains 0x2F ('/'). These previously caused
		// lookupKeyToRegisterID to misidentify the separator between key and height.
		{name: "0x2F in height encoding: with owner and key", height: heightWith0x2FInEncoding, owner: "owneraddress", key: "code.Token"},
		{name: "0x2F in height encoding: empty key", height: heightWith0x2FInEncoding, owner: "owneraddress", key: ""},
		{name: "0x2F in height encoding: empty owner", height: heightWith0x2FInEncoding, owner: "", key: "code.Token"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			encoded := newLookupKey(c.height, flow.RegisterID{Owner: c.owner, Key: c.key}).Bytes()
			decodedHeight, decodedReg, err := lookupKeyToRegisterID(encoded)
			require.NoError(t, err)
			require.Equal(t, c.height, decodedHeight)
			require.Equal(t, c.owner, decodedReg.Owner)
			require.Equal(t, c.key, decodedReg.Key)
		})
	}
}

// Test_decodeKey_errors tests all error paths of lookupKeyToRegisterID, and also verifies that
// valid inputs produce no error.
func Test_decodeKey_errors(t *testing.T) {
	cases := []struct {
		name     string
		key      []byte
		hasError bool
	}{
		{
			name:     "too few bytes",
			key:      []byte{codeRegister, 1, 2, 3, 4, 5, 6, 7, 8, 9}, // 10 bytes < MinLookupKeyLen (11)
			hasError: true,
		},
		{
			name:     "incorrect prefix",
			key:      []byte{^codeRegister, byte('/'), byte('/'), 1, 2, 3, 4, 5, 6, 7, 8},
			hasError: true,
		},
		{
			// After stripping the prefix we have 10 bytes. heightPos = 10-8 = 2, lookupKey[1] ≠ '/'
			name:     "separator not at expected position: key too short for height",
			key:      []byte{codeRegister, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			hasError: true,
		},
		{
			// After stripping the prefix we have 13 bytes. heightPos = 13-8 = 5, lookupKey[4] ≠ '/'
			name:     "separator not at expected position: one byte short of valid",
			key:      []byte{codeRegister, 1, 2, 3, '/', 5, '/', 7, 8, 9, 10, 11, 12, 13},
			hasError: true,
		},
		{
			// After stripping the prefix: {1,2,3,4,5,'/',7,8,9,10,11,12,13,14} (14 bytes).
			// heightPos = 6, lookupKey[5] = '/' passes the separator check.
			// The owner+key part {1,2,3,4,5} has no '/', so bytes.Cut fails.
			name:     "no slash between owner and key",
			key:      []byte{codeRegister, 1, 2, 3, 4, 5, '/', 7, 8, 9, 10, 11, 12, 13, 14},
			hasError: true,
		},
		{
			// After stripping the prefix: {1,2,3,'/',5,'/',7,8,9,10,11,12,13,14} (14 bytes).
			// heightPos = 6, lookupKey[5] = '/', owner+key = {1,2,3,'/',5} has '/'.
			name: "valid raw key",
			key:  []byte{codeRegister, 1, 2, 3, '/', 5, '/', 7, 8, 9, 10, 11, 12, 13, 14},
		},
		{
			name: "valid encoded key",
			key:  newLookupKey(uint64(0), flow.RegisterID{Owner: "owner", Key: "key"}).Bytes(),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := lookupKeyToRegisterID(c.key)
			if c.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
