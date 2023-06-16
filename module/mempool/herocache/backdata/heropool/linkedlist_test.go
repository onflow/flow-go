package heropool

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

// TestRemovingElementsFromList - first insures that list has been initialized correclty,
// then removes elements and tests lists consistency.
func TestRemovingElementsFromList(t *testing.T) {
	limit_capacity := uint32(4)
	number_of_entities := uint32(4)
	t.Run(fmt.Sprintf("%d-limit-%d-entities", limit_capacity, number_of_entities), func(t *testing.T) {
		withTestScenario(t, limit_capacity, number_of_entities, LRUEjection, []func(*testing.T, *Pool, []*unittest.MockEntity){
			func(t *testing.T, pool *Pool, entities []*unittest.MockEntity) {
				testRemovingElementsFromList(t, pool, entities, LRUEjection)
			},
		}...,
		)
	})
}

func checkList(t *testing.T, expectedList []EIndex, pool *Pool, list *state) {
	node_index := list.head
	i := 0
	// check size
	require.Equal(t, uint32(len(expectedList)), list.size)
	// check links
	for ; node_index != pool.free.tail; i++ {
		require.Equal(t, expectedList[i], node_index)
		node_index_next := pool.poolEntities[node_index].node.next
		require.Equal(t, expectedList[i], pool.poolEntities[node_index_next].node.prev)
		node_index = node_index_next
	}
	//check the tail
	require.Equal(t, expectedList[i], node_index)
}

func testRemovingElementsFromList(t *testing.T, pool *Pool, entitiesToBeAdded []*unittest.MockEntity, ejectionMode EjectionMode) {

	//check entities are initalized
	expectedList := [4]EIndex{0, 1, 2, 3}
	checkList(t, expectedList[:], pool, &pool.free)

	// remove middle
	expectedListNoMiddle := [3]EIndex{0, 1, 3}
	pool.free.removeEntity(pool, 2)
	checkList(t, expectedListNoMiddle[:], pool, &pool.free)

	// remove head
	pool.free.removeEntity(pool, 0)
	checkList(t, expectedListNoMiddle[1:], pool, &pool.free)

	// remove tail
	pool.free.removeEntity(pool, 3)
	checkList(t, expectedListNoMiddle[1:2], pool, &pool.free)

	// remove the last element and ty to remove from an empty list
	pool.free.removeEntity(pool, 1)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		} else {
			require.Equal(t, r, "Removing an entity from the empty list")
		}
	}()

	pool.free.removeEntity(pool, 1)
}

func testAppendingElementsToList(t *testing.T, pool *Pool, entitiesToBeAdded []*unittest.MockEntity, ejectionMode EjectionMode) {

	//check entities are initalized
	expectedList := [4]EIndex{0, 1, 2, 3}
	checkList(t, expectedList[:], pool, &pool.free)

	// add Elements
	expectedAfterAppendList := [3]EIndex{3, 0, 1}
	pool.used.appendEntity(pool, 3)
	pool.used.appendEntity(pool, 0)
	pool.used.appendEntity(pool, 1)
	checkList(t, expectedAfterAppendList[:], pool, &pool.used)
}
