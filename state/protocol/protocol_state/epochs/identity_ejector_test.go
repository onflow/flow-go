package epochs

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEjectorRapid performs a rapid check on the ejector structure, ensuring that it correctly tracks and ejects nodes.
// This test covers only happy-path scenario.
func TestEjectorRapid(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ej := ejector{}
		baseIdentities := unittest.DynamicIdentityEntryListFixture(5)
		// track 1-3 identity lists, each containing extra 0-7 identities
		trackedIdentities := rapid.Map(rapid.SliceOfN(rapid.IntRange(0, 7), 1, 3), func(n []int) []flow.DynamicIdentityEntryList {
			var result []flow.DynamicIdentityEntryList
			for _, count := range n {
				identities := append(baseIdentities.Copy(), unittest.DynamicIdentityEntryListFixture(count)...)
				identities = rapid.Permutation(identities).Draw(t, "shuffled-identities")
				result = append(result, identities)
			}
			return result
		}).Draw(t, "tracked-identities")

		for _, list := range trackedIdentities {
			err := ej.TrackDynamicIdentityList(list)
			require.NoError(t, err)
		}

		var ejectedIdentities []flow.Identifier
		for _, list := range trackedIdentities {
			nodeID := rapid.SampledFrom(list).Draw(t, "ejected-identity").NodeID
			require.True(t, ej.Eject(nodeID))
			ejectedIdentities = append(ejectedIdentities, nodeID)
		}

		for _, ejectedNodeID := range ejectedIdentities {
			ejected := false
			for _, list := range trackedIdentities {
				if entry, found := list.ByNodeID(ejectedNodeID); found && entry.Ejected {
					ejected = true
				}
			}
			require.True(t, ejected, "identity should be ejected in tracked list")
		}
	})
}

// TestEjector_ReadmitEjectedIdentity ensures that a node that was ejected cannot be readmitted with subsequent track requests.
func TestEjector_ReadmitEjectedIdentity(t *testing.T) {
	list := unittest.DynamicIdentityEntryListFixture(3)
	ej := ejector{}
	ejectedNodeID := list[0].NodeID
	require.NoError(t, ej.TrackDynamicIdentityList(list))
	require.True(t, ej.Eject(ejectedNodeID))
	readmit := append(unittest.DynamicIdentityEntryListFixture(3), &flow.DynamicIdentityEntry{
		NodeID:  ejectedNodeID,
		Ejected: false,
	})
	err := ej.TrackDynamicIdentityList(readmit)
	require.Error(t, err)
	require.True(t, protocol.IsInvalidServiceEventError(err))
}

// TestEjector_IdentityNotFound ensures that ejector returns false when the identity is not tracked.
// Tested different scenarios where the identity is not tracked.
func TestEjector_IdentityNotFound(t *testing.T) {
	t.Run("nothing-tracked", func(t *testing.T) {
		ej := ejector{}
		require.False(t, ej.Eject(unittest.IdentifierFixture()))
	})
	t.Run("list-tracked", func(t *testing.T) {
		ej := ejector{}
		require.NoError(t, ej.TrackDynamicIdentityList(unittest.DynamicIdentityEntryListFixture(3)))
		require.False(t, ej.Eject(unittest.IdentifierFixture()))
	})
	t.Run("after-ejection", func(t *testing.T) {
		ej := ejector{}
		list := unittest.DynamicIdentityEntryListFixture(3)
		require.NoError(t, ej.TrackDynamicIdentityList(list))
		require.True(t, ej.Eject(list[0].NodeID))
		require.False(t, ej.Eject(unittest.IdentifierFixture()))
	})
}
