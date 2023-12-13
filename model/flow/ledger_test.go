package flow

import (
	"encoding/hex"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/atree"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// this benchmark can run with this command:
//  go test -run=String -bench=.

// this is to prevent lint errors
var length int

func BenchmarkString(b *testing.B) {

	r := NewRegisterID(unittest.RandomAddressFixture(), "123412341234")

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

	r := NewRegisterID(unittest.RandomAddressFixture(), "123412341234")

	ret := fmt.Sprintf("%x/%x", r.Owner, r.Key)

	length = len(ret)
}

func TestRegisterID_IsInternalState(t *testing.T) {
	requireTrue := func(owner Address, key string) {
		id := NewRegisterID(owner, key)
		require.True(t, id.IsInternalState())
	}

	requireFalse := func(owner Address, key string) {
		id := NewRegisterID(owner, key)
		require.False(t, id.IsInternalState())
	}

	for i := 0; i < 256; i++ {
		uuid := UUIDRegisterID(byte(i))
		if i == 0 {
			require.Equal(t, uuid.Key, UUIDKeyPrefix)
			requireTrue(EmptyAddress, UUIDKeyPrefix)
		} else {
			require.Equal(t, uuid.Key, fmt.Sprintf("%s_%d", UUIDKeyPrefix, i))
			requireTrue(EmptyAddress, fmt.Sprintf("%s_%d", UUIDKeyPrefix, i))
		}
		require.True(t, uuid.IsInternalState())
	}
	require.True(t, AddressStateRegisterID.IsInternalState())
	requireTrue(EmptyAddress, AddressStateKey)
	requireFalse(EmptyAddress, "other")
	requireFalse(unittest.RandomAddressFixture(), UUIDKeyPrefix)
	requireFalse(unittest.RandomAddressFixture(), AddressStateKey)
	requireTrue(unittest.RandomAddressFixture(), "public_key_12")
	requireTrue(unittest.RandomAddressFixture(), ContractNamesKey)
	requireTrue(unittest.RandomAddressFixture(), "code.MYCODE")
	requireTrue(unittest.RandomAddressFixture(), AccountStatusKey)
	requireFalse(unittest.RandomAddressFixture(), "anything else")
}

func TestRegisterID_String(t *testing.T) {
	t.Run("atree slab", func(t *testing.T) {
		// slab with 189 should result in \\xbd
		slabIndex := atree.StorageIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 189})

		id := NewRegisterID(
			flow.BytesToAddress([]byte{1, 2, 3, 10}),
			string(atree.SlabIndexToLedgerKey(slabIndex)))
		require.False(t, utf8.ValidString(id.Key))
		printable := id.String()
		require.True(t, utf8.ValidString(printable))
		require.Equal(t, "000000000102030a/$189", printable)
	})

	t.Run("non slab invalid utf-8", func(t *testing.T) {
		id := NewRegisterID(flow.BytesToAddress([]byte("b\xc5y")), "a\xc5z")
		require.False(t, utf8.ValidString(id.Owner))
		require.False(t, utf8.ValidString(id.Key))
		printable := id.String()
		require.True(t, utf8.ValidString(printable))
		require.Equal(t, "000000000062c579/#61c57a", printable)
	})

	t.Run("global register", func(t *testing.T) {
		uuidRegisterID := UUIDRegisterID(0)
		id := NewRegisterID(EmptyAddress, uuidRegisterID.Key)
		require.Equal(t, uuidRegisterID.Owner, id.Owner)
		require.Equal(t, uuidRegisterID.Key, id.Key)
		printable := id.String()
		assert.True(t, utf8.ValidString(printable))
		assert.Equal(t, "/#75756964", printable)
	})
}
