package flow

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestIdentifierListSort tests the IdentityList against its implemented sort interface
// it generates and sorts a list of ids, and then evaluates sorting in ascending order
func TestIdentifierListSort(t *testing.T) {
	count := 10
	// creates an identity list of 10 ids
	var identityList IdentityList
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &Identity{
			NodeID: nodeID,
		}
		identityList = append(identityList, identity)
	}
	var ids IdentifierList = identityList.NodeIDs()

	// shuffles array before sorting to enforce some pseudo-randomness
	rand.Seed(time.Now().UnixNano())
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
