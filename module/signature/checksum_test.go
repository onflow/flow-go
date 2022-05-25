package signature_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test that the CheckSumFromIdentities method is able to produce checksum for empty identity list,
// and produce the correct checksum
func TestCheckSum(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		require.Equal(t, signature.CheckSumFromIdentities(nil), signature.CheckSumFromIdentities(nil))
		require.Equal(t, signature.CheckSumFromIdentities([]flow.Identifier{}), signature.CheckSumFromIdentities([]flow.Identifier{}))
		require.Equal(t, signature.CheckSumFromIdentities(nil), signature.CheckSumFromIdentities([]flow.Identifier{}))
	})

	t.Run("same identities, same checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(3)
		require.Equal(t, signature.CheckSumFromIdentities(ids), signature.CheckSumFromIdentities(ids))
		require.Equal(t, signature.CheckSumFromIdentities(ids[1:]), signature.CheckSumFromIdentities(ids[1:]))
		require.Equal(t, signature.CheckSumFromIdentities(ids[2:]), signature.CheckSumFromIdentities(ids[2:]))
	})

	t.Run("different identities, different checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.NotEqual(t, signature.CheckSumFromIdentities(ids), signature.CheckSumFromIdentities(ids[1:]))     // subset
		require.NotEqual(t, signature.CheckSumFromIdentities(ids[1:]), signature.CheckSumFromIdentities(ids[:2])) // overlap
		require.NotEqual(t, signature.CheckSumFromIdentities(ids[:2]), signature.CheckSumFromIdentities(ids[2:])) // no overlap
	})

	t.Run("checksum length always constant", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.Len(t, signature.CheckSumFromIdentities(nil), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[1:]), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[2:]), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[3:]), signature.CheckSumLen)
	})
}

// Test that if an encoder generates a checksum with a committee and added to some random data
// using PrefixCheckSum method, then an decoder using the same committee to call CompareAndExtract
// is able to extract the same data as the encoder.
func TestPrefixCheckSum(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		committeeSize := rapid.IntRange(0, 300).Draw(t, "committeeSize").(int)
		committee := unittest.IdentifierListFixture(committeeSize)
		data := rapid.IntRange(0, 200).Map(func(count int) []byte {
			return unittest.RandomBytes(count)
		}).Draw(t, "data").([]byte)
		extracted, err := signature.CompareAndExtract(committee, signature.PrefixCheckSum(committee, data))
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
			_, _, err := signature.SplitCheckSum(unittest.RandomBytes(i))
			require.True(t, errors.Is(err, signature.ErrInvalidChecksum))
		}
	})

	t.Run("mismatching checksum", func(t *testing.T) {
		committee := unittest.IdentifierListFixture(20)
		_, err := signature.CompareAndExtract(committee, unittest.RandomBytes(112))
		require.True(t, errors.Is(err, signature.ErrInvalidChecksum))
	})
}
