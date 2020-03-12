package flow_test

import (
	"bytes"
	"math/rand"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TestIdentifierList_Sort tests the IdentityList against its implemented sort interface
// it generates and sorts a list of ids, and then evaluates sorting in ascending order
func TestIdentifierList_Sort(t *testing.T) {

	count := 10
	// creates an identity list of 10 ids
	var identityList flow.IdentityList
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &flow.Identity{
			NodeID: nodeID,
		}
		identityList = append(identityList, identity)
	}
	var ids flow.IdentifierList = identityList.NodeIDs()

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

// TestIdentifierList_Join tests correctness of joining two IdentityLists
func TestIdentifierList_Join(t *testing.T) {

	// creates an identity list of 10 ids
	count := 10
	var identityList flow.IdentityList
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &flow.Identity{
			NodeID: nodeID,
		}
		identityList = append(identityList, identity)
	}
	var ids flow.IdentifierList = identityList.NodeIDs()

	// breaks the IdentityList into two parts
	part1 := ids[:count/2]
	part2 := ids[count/2:]

	// joins two parts back together
	joined := part1.Join(part2)

	// joined should have the same length and elements as
	// the original one
	require.Equal(t, ids.Len(), joined.Len())
	assert.Equal(t, ids, joined)

	// reverse join swaps part 1 and 2 on joining
	reversed := part2.Join(part1)
	// joined should have the same length as the original
	require.Equal(t, ids.Len(), reversed.Len())
	// reversed join should not be the same as the original
	// in the order of elements, but equal in their set of elements
	assert.NotEqual(t, ids, reversed)
	assert.ElementsMatch(t, ids, reversed)
}

func TestIdentifierList_RandSubsetNRandSubset(t *testing.T) {

	t.Run("n<0", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		subset := list.RandSubsetN(-1)
		assert.Len(t, subset, 0)
	})

	t.Run("n>len(L)", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		subset := list.RandSubsetN(11)
		assert.Len(t, subset, 10)
	})

	t.Run("subsets", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			list := unittest.IdentifierListFixture(10)
			size := rand.Intn(10)
			subset := list.RandSubsetN(size)

			// should be right size
			assert.Len(t, subset, size)

			// should contain no duplicates
			lookup := make(map[flow.Identifier]struct{})
			for _, id := range subset {
				if _, exists := lookup[id]; exists {
					t.Log("duplicate id in subset: ", id)
					t.Fail()
				}
				lookup[id] = struct{}{}
			}

			// should only contain elements from original list
			for _, id := range subset {
				assert.Contains(t, list, id)
			}
		}
	})
}

func TestIdentifierList_With(t *testing.T) {

	// should not double-add ID
	t.Run("existing id", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		with := list.With(list[0])
		assert.Len(t, with, 10)
		assert.Equal(t, list, with)
	})

	// should add new ID
	t.Run("new id", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		id := unittest.IdentifierFixture()

		with := list.With(id)
		assert.Len(t, with, 11)
		assert.Contains(t, with, id)
		for _, id := range list {
			assert.Contains(t, with, id)
		}
	})

	// should only same ID once
	t.Run("duplicate ID", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		id := unittest.IdentifierFixture()

		with := list.With(id, id)
		assert.Len(t, with, 11)
		assert.Contains(t, with, id)
		for _, id := range list {
			assert.Contains(t, with, id)
		}
	})

	// should handle more complex inputs
	t.Run("some new, some existing ids", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		newID1 := unittest.IdentifierFixture()
		newID2 := unittest.IdentifierFixture()

		with := list.With(list[0], list[3], list[7], newID1, newID2)
		assert.Len(t, with, 12)
		assert.Contains(t, with, newID1)
		assert.Contains(t, with, newID2)
		for _, id := range list {
			assert.Contains(t, with, id)
		}
	})
}

func TestIdentifierList_Without(t *testing.T) {

	// should remove existing id
	t.Run("existing id", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		without := list.Without(list[0])
		assert.Len(t, without, 9)
		assert.NotContains(t, without, list[0])
		for _, id := range list[1:] {
			assert.Contains(t, without, id)
		}
	})

	// should have no effect
	t.Run("non-existent id", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		id := unittest.IdentifierFixture()

		without := list.Without(id)
		assert.Len(t, without, 10)
		assert.Equal(t, list, without)
	})

	// should handle more complex inputs
	t.Run("some existing, some non-existent ids", func(t *testing.T) {
		list := unittest.IdentifierListFixture(10)
		newID1 := unittest.IdentifierFixture()
		newID2 := unittest.IdentifierFixture()

		with := list.Without(list[1], list[4], list[8], newID1, newID2)
		assert.Len(t, with, 7)
		assert.NotContains(t, with, list[1])
		assert.NotContains(t, with, list[4])
		assert.NotContains(t, with, list[8])
		for i, id := range list {
			if i == 1 || i == 4 || i == 8 {
				continue
			}
			assert.Contains(t, with, id)
		}
	})
}
