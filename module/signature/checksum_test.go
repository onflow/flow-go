package signature_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that the CheckSumFromIdentities method is able to produce checksum for empty identity list,
// and produce the correct checksum
func TestCheckSum(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		require.Equal(t, msig.CheckSumFromIdentities(nil), msig.CheckSumFromIdentities(nil))
		require.Equal(t, msig.CheckSumFromIdentities([]flow.Identifier{}), msig.CheckSumFromIdentities([]flow.Identifier{}))
		require.Equal(t, msig.CheckSumFromIdentities(nil), msig.CheckSumFromIdentities([]flow.Identifier{}))
	})

	t.Run("same identities, same checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(3)
		require.Equal(t, msig.CheckSumFromIdentities(ids), msig.CheckSumFromIdentities(ids))
		require.Equal(t, msig.CheckSumFromIdentities(ids[1:]), msig.CheckSumFromIdentities(ids[1:]))
		require.Equal(t, msig.CheckSumFromIdentities(ids[2:]), msig.CheckSumFromIdentities(ids[2:]))
	})

	t.Run("different identities, different checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.NotEqual(t, msig.CheckSumFromIdentities(ids), msig.CheckSumFromIdentities(ids[1:]))     // subset
		require.NotEqual(t, msig.CheckSumFromIdentities(ids[1:]), msig.CheckSumFromIdentities(ids[:2])) // overlap
		require.NotEqual(t, msig.CheckSumFromIdentities(ids[:2]), msig.CheckSumFromIdentities(ids[2:])) // no overlap
	})

	t.Run("checksum length always constant", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.Len(t, msig.CheckSumFromIdentities(nil), msig.CheckSumLen)
		require.Len(t, msig.CheckSumFromIdentities(ids), msig.CheckSumLen)
		require.Len(t, msig.CheckSumFromIdentities(ids[1:]), msig.CheckSumLen)
		require.Len(t, msig.CheckSumFromIdentities(ids[2:]), msig.CheckSumLen)
		require.Len(t, msig.CheckSumFromIdentities(ids[3:]), msig.CheckSumLen)
	})
}

// Test that if an encoder generates a checksum with a committee and added to some random data
// using PrefixCheckSum method, then an decoder using the same committee to call CompareAndExtract
// is able to extract the same data as the encoder.
func TestPrefixCheckSum(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		committeeSize := rapid.IntRange(0, 300).Draw(t, "committeeSize")
		committee := unittest.IdentifierListFixture(committeeSize)
		data := rapid.Map(rapid.IntRange(0, 200), func(count int) []byte {
			return unittest.RandomBytes(count)
		}).Draw(t, "data")
		extracted, err := msig.CompareAndExtract(committee, msig.PrefixCheckSum(committee, data))
		require.NoError(t, err)
		require.Equal(t, data, extracted)
	})
}

// Test_InvalidCheckSum verifies correct handling of invalid checksums. We expect:
//  1. `SplitCheckSum` returns the expected sentinel error `ErrInvalidChecksum` is the input is shorter than 4 bytes
//  2. `CompareAndExtract` returns `ErrInvalidChecksum` is the checksum does not match
func Test_InvalidCheckSum(t *testing.T) {
	t.Run("checksum too short", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			_, _, err := msig.SplitCheckSum(unittest.RandomBytes(i))
			require.True(t, errors.Is(err, msig.ErrInvalidChecksum))
		}
	})

	t.Run("mismatching checksum", func(t *testing.T) {
		committee := unittest.IdentifierListFixture(20)
		_, err := msig.CompareAndExtract(committee, unittest.RandomBytes(112))
		require.True(t, errors.Is(err, msig.ErrInvalidChecksum))
	})
}
