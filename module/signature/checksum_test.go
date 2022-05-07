package signature_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

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

	t.Run("checksum length always 16", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.Len(t, signature.CheckSumFromIdentities(nil), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[1:]), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[2:]), signature.CheckSumLen)
		require.Len(t, signature.CheckSumFromIdentities(ids[3:]), signature.CheckSumLen)
	})
}

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
