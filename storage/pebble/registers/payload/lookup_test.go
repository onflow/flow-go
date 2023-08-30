package payload

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/stretchr/testify/require"
)

// Test_lookupKey_Bytes tests the lookup key encoding.
func Test_lookupKey_Bytes(t *testing.T) {
	t.Parallel()

	expectedHeight := uint64(777)
	key := newLookupKey(expectedHeight, flow.RegisterID{Owner: "owner", Key: "key"})

	// Test encoded Owner and Key
	require.Equal(t, []byte("owner/key/"), key.Bytes()[:10])

	// Test encoded height
	actualHeight := binary.BigEndian.Uint64(key.Bytes()[10:])
	require.Equal(t, math.MaxUint64-actualHeight, expectedHeight)

	// Test everything together
	require.Equal(t, []byte("owner/key/\xff\xff\xff\xff\xff\xff\xfc\xf6"), key.Bytes())

	decodedHeight, decodedReg, err := lookupKeyToRegisterID(key.encoded)
	require.NoError(t, err)

	require.Equal(t, expectedHeight, decodedHeight)
	require.Equal(t, "owner", decodedReg.Owner)
	require.Equal(t, "key", decodedReg.Key)
}

func Test_decodeKey_Bytes(t *testing.T) {
	height := uint64(10)

	cases := []struct {
		owner string
		key   string
	}{
		{owner: "owneraddress", key: "public/storage/hasslash-in-key"},
		{owner: "owneraddress", key: ""},
		{owner: "", key: "somekey"},
		{owner: "", key: ""},
	}

	for _, c := range cases {
		owner, key := c.owner, c.key

		lookupKey := newLookupKey(height, flow.RegisterID{Owner: owner, Key: key})
		decodedHeight, decodedReg, err := lookupKeyToRegisterID(lookupKey.Bytes())
		require.NoError(t, err)

		require.Equal(t, height, decodedHeight)
		require.Equal(t, owner, decodedReg.Owner)
		require.Equal(t, key, decodedReg.Key)
	}
}

func Test_decodeKey_fail(t *testing.T) {
	var err error
	// less than min length (10)
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9})
	require.Contains(t, err.Error(), "bytes")

	// missing slash
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
	require.Contains(t, err.Error(), "slash")

	// missing second slash
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, '/', 5, 6, 7, 8, 9, 10})
	require.Contains(t, err.Error(), "separator")

	// invalid height
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, '/', 5, 6, 7, 8, '/', 10})
	require.Contains(t, err.Error(), "height")

	// invalid height
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, '/', 5, '/', 7, 8, 9, 10})
	require.Contains(t, err.Error(), "height")

	// invalid height
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, '/', 5, '/', 7, 8, 9, 10, 11, 12, 13})
	require.Contains(t, err.Error(), "height")

	// valid height
	_, _, err = lookupKeyToRegisterID([]byte{1, 2, 3, '/', 5, '/', 7, 8, 9, 10, 11, 12, 13, 14})
	require.NoError(t, err)
}
