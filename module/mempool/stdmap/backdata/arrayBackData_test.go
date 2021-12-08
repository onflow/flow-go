package backdata

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/arraylinkedlist"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestArrayBackData_BelowLimit(t *testing.T) {
	size := 10

	bd := NewArrayBackData(uint32(size), 1, arraylinkedlist.LRUEjection)

	entities := unittest.EntityListFixture(uint(size))

	// adding elements
	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, bd.Add(e.ID(), e))

		// total of back data should be incremented by each addition.
		require.Equal(t, bd.Size(), uint(i+1))

		// entity should be placed at index i in back data
		id, entity, _ := bd.entities.Get(uint32(i))
		require.Equal(t, e.ID(), id)
		require.Equal(t, e, entity)
	}

	// sanity checks
	for i := range entities {
		// since we are below limit, elements should be added sequentially at bucket 0.
		// first added element has a key index of 1, since 0 means unused key index in implementation.
		require.Equal(t, bd.buckets[0][i].keyIndex, uint64(i+1))
		// also, since we have not yet over-limited, entities are received valueIndex in the same order they
		// are added.
		require.Equal(t, bd.buckets[0][i].valueIndex, uint32(i))
		_, _, owner := bd.entities.Get(uint32(i))
		require.Equal(t, owner, uint64(i))
	}

	// getting inserted elements
	for _, expected := range entities {
		actual, ok := bd.ByID(expected.ID())
		require.True(t, ok)
		require.Equal(t, expected, actual)
	}
}

func TestInvalidateEntity(t *testing.T) {
	for _, tc := range []struct {
		limit           uint32
		overLimitFactor uint32
		entityCount     uint32
		helpers         []func(*testing.T, *ArrayBackData, []*unittest.MockEntity)
	}{
		{ // edge-case empty list
			limit:           30,
			overLimitFactor: 2,
			entityCount:     0,
		},
		{ // edge-case single element-list
			limit:           30,
			overLimitFactor: 2,
			entityCount:     1,
		},
		{ // two buckets, entities below limit.
			limit:           30,
			overLimitFactor: 2,
			entityCount:     10,
		},
		{ // two buckets, entities equal to limit.
			limit:           30,
			overLimitFactor: 2,
			entityCount:     30,
		},
		{ // multiple buckets, high limit, low entities.
			limit:           100,
			overLimitFactor: 16,
			entityCount:     10,
		},
		{ // multiple buckets, entities equal to limit.
			limit:           100,
			overLimitFactor: 16,
			entityCount:     100,
		},
	} {
		// head invalidation test (LRU)
		t.Run(fmt.Sprintf("head-invalidation-%d-limit-%d-overlimit-%d-entities", tc.limit, tc.overLimitFactor, tc.entityCount), func(t *testing.T) {
			testInvalidateEntity(t, tc.limit, tc.overLimitFactor, tc.entityCount, func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
				testInvalidatingHead(t, backData, entities)
			})
		})

		// tail invalidation test
		t.Run(fmt.Sprintf("tail-invalidation-%d-limit-%d-overlimit-%d-entities-", tc.limit, tc.overLimitFactor, tc.entityCount), func(t *testing.T) {
			testInvalidateEntity(t, tc.limit, tc.overLimitFactor, tc.entityCount, func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
				testInvalidatingTail(t, backData, entities)
			})
		})

		// random invalidation test
		t.Run(fmt.Sprintf("random-invalidation-%d-limit-%d-overlimit-%d-entities-", tc.limit, tc.overLimitFactor, tc.entityCount),
			func(t *testing.T) {
				testInvalidateEntity(t, tc.limit, tc.overLimitFactor, tc.entityCount, func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
					testInvalidateAtRandom(t, backData, entities)
				})
			})
	}
}

func testInvalidateEntity(t *testing.T, limit uint32, overLimitFactor uint32, entityCount uint32, helpers ...func(*testing.T, *ArrayBackData, []*unittest.MockEntity)) {
	h := append([]func(*testing.T, *ArrayBackData, []*unittest.MockEntity){
		func(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
			testAddingEntities(t, backData, entities)
		},
	}, helpers...)

	withTestScenario(t, limit, overLimitFactor, entityCount, h...)
}

//
//// testInvalidatingHead keeps invalidating elements at random and evaluates whether double-linked list remains
//// connected on both head and tail.
//func testInvalidateAtRandom(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
//	size := len(entities)
//	offset := int(backData.limit) - size
//
//	for i := 0; i < size; i++ {
//		backData.entities.invalidateRandomEntity()
//
//		// size of list should be shrunk after each invalidation.
//		require.Equal(t, uint32(size-i-1), backData.entities.size())
//
//		// except when the list is empty, head and tail must be accessible after each invalidation
//		// i.e., the linked list remains connected despite invalidation.
//		if i != size-1 {
//			// used list
//			tailAccessibleFromHead(t,
//				backData.entities.used.head.sliceIndex(),
//				backData.entities.used.tail.sliceIndex(),
//				backData,
//				backData.entities.size())
//
//			headAccessibleFromTail(t,
//				backData.entities.used.head.sliceIndex(),
//				backData.entities.used.tail.sliceIndex(),
//				backData,
//				backData.entities.size())
//
//			// free list
//			headAccessibleFromTail(t,
//				backData.entities.free.head.sliceIndex(),
//				backData.entities.free.tail.sliceIndex(),
//				backData,
//				uint32(i+1+offset))
//
//			tailAccessibleFromHead(t,
//				backData.entities.free.head.sliceIndex(),
//				backData.entities.free.tail.sliceIndex(),
//				backData,
//				uint32(i+1+offset))
//		}
//	}
//}
//
//// testInvalidatingHead keeps invalidating the head and evaluates the linked-list keeps updating its head
//// and remains connected.
//func testInvalidatingHead(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
//	size := len(entities)
//	for i := 0; i < size; i++ {
//		headIndex := backData.entities.invalidateHead()
//		require.Equal(t, uint32(i), headIndex)
//
//		// size of list should be shrunk after each invalidation.
//		require.Equal(t, uint32(size-i-1), backData.entities.size())
//		// unclaimed head should be appended to free entities
//		require.Equal(t, backData.entities.free.tail.sliceIndex(), headIndex)
//
//		// except when the list is empty, head must be updated after invalidation,
//		// except when the list is empty, head and tail must be accessible after each invalidation
//		// i.e., the linked list remains connected despite invalidation.
//		if i != size-1 {
//			// require.Equal(t, entities[i+1].ID(), backData.entities.getHead().id)
//			tailAccessibleFromHead(t,
//				backData.entities.used.head.sliceIndex(),
//				backData.entities.used.tail.sliceIndex(),
//				backData,
//				backData.entities.size())
//			// headAccessibleFromTail(t, backData, backData.entities.size())
//		}
//	}
//}
//
//// testInvalidatingHead keeps invalidating the tail and evaluates the linked-list keeps updating its tail
//// and remains connected.
//func testInvalidatingTail(t *testing.T, backData *ArrayBackData, entities []*unittest.MockEntity) {
//	size := len(entities)
//	for i := 0; i < size; i++ {
//		// invalidates tail index
//		tail := backData.entities.used.tail.sliceIndex()
//		backData.entities.invalidateEntityAtIndex(tail)
//		// old head index must be invalidated
//		require.True(t, backData.entities.isInvalidated(tail))
//
//		// size of list should be shrunk after each invalidation.
//		require.Equal(t, uint32(size-i-1), backData.entities.size())
//
//		// except when the list is empty, tail must be updated after invalidation,
//		// and also head and tail must be accessible after each invalidation
//		// i.e., the linked list remains connected despite invalidation.
//		if i != size-1 {
//			// require.Equal(t, entities[size-i-2].ID(), backData.entities.getTail().id)
//			tailAccessibleFromHead(t,
//				backData.entities.used.head.sliceIndex(),
//				backData.entities.used.tail.sliceIndex(),
//				backData,
//				backData.entities.size())
//			// headAccessibleFromTail(t, backData, backData.entities.size())
//		}
//	}
//}
//
