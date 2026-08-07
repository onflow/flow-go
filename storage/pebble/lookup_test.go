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
			owner:  "owner678", // 8 bytes (flow.AddressLength)
			key:    "key",
			// [codeRegister] + "owner678/key/" + ^777 as big-endian uint64
			expectedBytes: append([]byte{codeRegister}, []byte("owner678/key/\xff\xff\xff\xff\xff\xff\xfc\xf6")...),
		},
		{
			name:   "height 0 encodes as all 0xff",
			height: 0,
			owner:  "aaaaaaaa",
			key:    "b",
			// ^0 = MaxUint64 = 0xFFFFFFFFFFFFFFFF
			expectedBytes: append([]byte{codeRegister}, []byte("aaaaaaaa/b/\xff\xff\xff\xff\xff\xff\xff\xff")...),
		},
		{
			name:   "max height encodes as all 0x00",
			height: math.MaxUint64,
			owner:  "aaaaaaaa",
			key:    "b",
			// ^MaxUint64 = 0
			expectedBytes: append([]byte{codeRegister}, []byte("aaaaaaaa/b/\x00\x00\x00\x00\x00\x00\x00\x00")...),
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
		{name: "key with slash", height: 10, owner: "owner678", key: "public/storage/hasslash-in-key"},
		{name: "empty key", height: 10, owner: "owner678", key: ""},
		{name: "empty owner", height: 10, owner: "", key: "somekey"},
		{name: "empty owner and key", height: 10, owner: "", key: ""},
		// Heights whose one's complement encoding contains 0x2F ('/'). These previously caused
		// lookupKeyToRegisterID to misidentify the separator between key and height.
		{name: "0x2F in height encoding: with owner and key", height: heightWith0x2FInEncoding, owner: "owner678", key: "code.Token"},
		{name: "0x2F in height encoding: empty key", height: heightWith0x2FInEncoding, owner: "owner678", key: ""},
		{name: "0x2F in height encoding: empty owner", height: heightWith0x2FInEncoding, owner: "", key: "code.Token"},
		// Owners are raw address bytes and may contain 0x2F ('/'). These previously caused
		// lookupKeyToRegisterID to split the owner at the first '/' byte (issue #8617).
		{name: "0x2F in owner: first byte", height: 10, owner: "/abcdefg", key: "code.Token"},
		{name: "0x2F in owner: middle byte", height: 10, owner: "ab/cdefg", key: "contract_names"},
		{name: "0x2F in owner: last byte", height: 10, owner: "abcdefg/", key: "somekey"},
		{name: "0x2F in owner: all bytes", height: 10, owner: "////////", key: "somekey"},
		{name: "0x2F in owner: empty key", height: 10, owner: "ab/cdefg", key: ""},
		{name: "0x2F in owner and height encoding", height: heightWith0x2FInEncoding, owner: "ab/cdefg", key: "code.Token"},
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
			// owner+key part {1,2,3,4,5} has no '/' at a valid owner boundary (0 or flow.AddressLength)
			name:     "no owner/key separator",
			key:      []byte{codeRegister, 1, 2, 3, 4, 5, '/', 7, 8, 9, 10, 11, 12, 13, 14},
			hasError: true,
		},
		{
			// owner+key part {1,2,3,'/',5} has a '/', but not at a valid owner boundary
			name:     "separator at wrong owner boundary",
			key:      []byte{codeRegister, 1, 2, 3, '/', 5, '/', 7, 8, 9, 10, 11, 12, 13, 14},
			hasError: true,
		},
		{
			// owner is exactly flow.AddressLength (8) bytes followed by '/'; key is {9}.
			name: "valid raw key with 8-byte owner",
			key:  []byte{codeRegister, 1, 2, 3, 4, 5, 6, 7, 8, '/', 9, '/', 10, 11, 12, 13, 14, 15, 16, 17},
		},
		{
			name: "valid encoded key",
			key:  newLookupKey(uint64(0), flow.RegisterID{Owner: "owner678", Key: "key"}).Bytes(),
		},
		{
			// owner is neither empty nor flow.AddressLength bytes, so it cannot be decoded
			name:     "owner with invalid length",
			key:      newLookupKey(uint64(0), flow.RegisterID{Owner: "owner", Key: "key"}).Bytes(),
			hasError: true,
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
