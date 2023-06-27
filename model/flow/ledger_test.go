package flow

import (
	"encoding/hex"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"
)

// this benchmark can run with this command:
//  go test -run=String -bench=.

// this is to prevent lint errors
var length int

func BenchmarkString(b *testing.B) {

	r := NewRegisterID("theowner", "123412341234")

	ownerLen := len(r.Owner)

	requiredLen := ((ownerLen + len(r.Key)) * 2) + 1

	arr := make([]byte, requiredLen)

	hex.Encode(arr, []byte(r.Owner))

	arr[2*ownerLen] = byte('/')

	hex.Encode(arr[(2*ownerLen)+1:], []byte(r.Key))

	s := string(arr)
	length = len(s)
}

func BenchmarkOriginalString(b *testing.B) {

	r := NewRegisterID("theowner", "123412341234")

	ret := fmt.Sprintf("%x/%x", r.Owner, r.Key)

	length = len(ret)
}

func TestRegisterID_IsInternalState(t *testing.T) {
	requireTrue := func(owner string, key string) {
		id := NewRegisterID(owner, key)
		require.True(t, id.IsInternalState())
	}

	requireFalse := func(owner string, key string) {
		id := NewRegisterID(owner, key)
		require.False(t, id.IsInternalState())
	}

	for i := 0; i < 256; i++ {
		uuid := UUIDRegisterID(byte(i))
		if i == 0 {
			require.Equal(t, uuid.Key, UUIDKeyPrefix)
		} else {
			require.Equal(t, uuid.Key, fmt.Sprintf("%s_%d", UUIDKeyPrefix, i))
		}
		require.True(t, uuid.IsInternalState())
	}
	requireFalse("", UUIDKeyPrefix)
	for i := 0; i < 256; i++ {
		requireFalse("", fmt.Sprintf("%s_%d", UUIDKeyPrefix, i))
	}
	require.True(t, AddressStateRegisterID.IsInternalState())
	requireFalse("", AddressStateKey)
	requireFalse("", "other")
	requireFalse("Address", UUIDKeyPrefix)
	requireFalse("Address", AddressStateKey)
	requireTrue("Address", "public_key_12")
	requireTrue("Address", ContractNamesKey)
	requireTrue("Address", "code.MYCODE")
	requireTrue("Address", AccountStatusKey)
	requireFalse("Address", "anything else")
}

func TestRegisterID_String(t *testing.T) {
	// slab with 189 should result in \\xbd
	slabIndex := atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 189})

	id := NewRegisterID(
		string([]byte{1, 2, 3, 10}),
		string(atree.SlabIndexToLedgerKey(slabIndex)))
	require.False(t, utf8.ValidString(id.Key))
	printable := id.String()
	require.True(t, utf8.ValidString(printable))
	require.Equal(t, "000000000102030a/$189", printable)

	// non slab invalid utf-8
	id = NewRegisterID("b\xc5y", "a\xc5z")
	require.False(t, utf8.ValidString(id.Owner))
	require.False(t, utf8.ValidString(id.Key))
	printable = id.String()
	require.True(t, utf8.ValidString(printable))
	require.Equal(t, "000000000062c579/#61c57a", printable)
}
