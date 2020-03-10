package flow

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/network/gossip/libp2p/test"
)

// TestIdentifierListSort tests the IdentityList against its implemented sort interface
// it generates and sorts a list of ids, and then evaluates sorting in ascending order
func TestIdentifierListSort(t *testing.T) {
	// creates an identity list of 10 ids
	var idList IdentityList = test.CreateIDs(10)
	var ids IdentifierList = idList.NodeIDs()

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

// TestJoinIdentifierLists tests correctness of joining two IdentityLists
func TestJoinIdentifierLists(t *testing.T) {
	// creates an identity list of 10 ids
	count := 10
	var idList IdentityList = test.CreateIDs(count)
	var ids IdentifierList = idList.NodeIDs()

	// breaks the IdentityList into two parts
	part1 := ids[:count/2]
	part2 := ids[count/2:]

	// joins two parts back together
	joined := JoinIdentifierLists(part1, part2)

	// joined should have the same length and elements as
	// the original one
	require.Equal(t, ids.Len(), joined.Len())
	assert.Equal(t, ids, joined)

	// reverse join swaps part 1 and 2 on joining
	reversed := JoinIdentifierLists(part2, part1)
	// joined should have the same length as the original
	require.Equal(t, ids.Len(), reversed.Len())
	// reversed join should not be the same as the original
	// in the order of elements, but equal in their set of elements
	assert.NotEqual(t, ids, reversed)
	assert.ElementsMatch(t, ids, reversed)

}
