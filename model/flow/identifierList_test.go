package flow_test

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test the canonical ordering of identity and identifier match
func TestCanonicalOrderingMatch(t *testing.T) {
	identities := unittest.IdentityListFixture(100)
	require.Equal(t,
		identities.Sort(flow.Canonical).NodeIDs(),
		identities.NodeIDs().Sort(flow.IdentifierCanonical))
}

// TestIdentifierListSort tests the IdentityList against its implemented sort interface
// it generates and sorts a list of ids, and then evaluates sorting in ascending order
func TestIdentifierListSort(t *testing.T) {
	count := 10
	// creates an identifier list of 10 ids
	var ids flow.IdentifierList = unittest.IdentifierListFixture(count)

	// shuffles array before sorting to enforce some pseudo-randomness

	rand.Shuffle(ids.Len(), ids.Swap)

	sort.Sort(ids)

	before := ids[0]
	// compares each id being greater than or equal to its previous one
	// on the sorted list
	for _, id := range ids {
		if bytes.Compare(id[:], before[:]) == -1 {
			// test fails due to id < before which is in contrast to the
			// ascending order assumption of sort
			require.Fail(t, "sort does not work in ascending order")
		}
		before = id
	}
}

// TestIdentifierListContains tests the IdentifierList against its Contains method implementation.
func TestIdentifierListContains(t *testing.T) {
	count := 10
	// creates an identifier list of 10 ids
	var ids flow.IdentifierList = unittest.IdentifierListFixture(count)

	// all identifiers in the list should have a valid Contains result.
	for _, id := range ids {
		require.True(t, ids.Contains(id))
	}

	// non-existent identifier should have a negative Contains result.
	nonExistent := unittest.IdentifierFixture()
	require.False(t, ids.Contains(nonExistent))
}
