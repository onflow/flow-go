package signature

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestCheckSum(t *testing.T) {
	t.Run("no identity", func(t *testing.T) {
		require.Equal(t, CheckSumFromIdentities(nil), CheckSumFromIdentities(nil))
		require.Equal(t, CheckSumFromIdentities([]flow.Identifier{}), CheckSumFromIdentities([]flow.Identifier{}))
		require.Equal(t, CheckSumFromIdentities(nil), CheckSumFromIdentities([]flow.Identifier{}))
	})

	t.Run("same identities, same checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(3)
		require.Equal(t, CheckSumFromIdentities(ids), CheckSumFromIdentities(ids))
		require.Equal(t, CheckSumFromIdentities(ids[1:]), CheckSumFromIdentities(ids[1:]))
		require.Equal(t, CheckSumFromIdentities(ids[2:]), CheckSumFromIdentities(ids[2:]))
	})

	t.Run("different identities, different checksum", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.NotEqual(t, CheckSumFromIdentities(ids), CheckSumFromIdentities(ids[1:]))     // subset
		require.NotEqual(t, CheckSumFromIdentities(ids[1:]), CheckSumFromIdentities(ids[:2])) // overlap
		require.NotEqual(t, CheckSumFromIdentities(ids[:2]), CheckSumFromIdentities(ids[2:])) // no overlap
	})

	t.Run("checksum length always 16", func(t *testing.T) {
		ids := unittest.IdentifierListFixture(4)
		require.Len(t, CheckSumFromIdentities(nil), CheckSumLen)
		require.Len(t, CheckSumFromIdentities(ids), CheckSumLen)
		require.Len(t, CheckSumFromIdentities(ids[1:]), CheckSumLen)
		require.Len(t, CheckSumFromIdentities(ids[2:]), CheckSumLen)
		require.Len(t, CheckSumFromIdentities(ids[3:]), CheckSumLen)
	})
}
