package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestQueue(t *testing.T) {

	/* Input queue:
	  g-b
	     \
	f--d--c-a
	  /
	 e

	*/

	a := unittest.ExecutableBlockFixture(nil)
	c := unittest.ExecutableBlockFixtureWithParent(nil, a.Block.Header)
	b := unittest.ExecutableBlockFixtureWithParent(nil, c.Block.Header)
	d := unittest.ExecutableBlockFixtureWithParent(nil, c.Block.Header)
	e := unittest.ExecutableBlockFixtureWithParent(nil, d.Block.Header)
	f := unittest.ExecutableBlockFixtureWithParent(nil, d.Block.Header)
	g := unittest.ExecutableBlockFixtureWithParent(nil, b.Block.Header)

	dBroken := unittest.ExecutableBlockFixtureWithParent(nil, c.Block.Header)
	dBroken.Block.Header.Height += 2 //change height

	queue := NewQueue(a)

	t.Run("Adding", func(t *testing.T) {
		stored, _ := queue.TryAdd(b) //parent not stored yet
		size := queue.Size()
		height := queue.Height()
		assert.False(t, stored)
		assert.Equal(t, 1, size)
		assert.Equal(t, uint64(0), height)

		stored, new := queue.TryAdd(c)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 2, size)
		assert.Equal(t, uint64(1), height)

		stored, new = queue.TryAdd(b)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 3, size)
		assert.Equal(t, uint64(2), height)

		stored, new = queue.TryAdd(b) //repeat
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.False(t, new)
		assert.Equal(t, 3, size)
		assert.Equal(t, uint64(2), height)

		stored, _ = queue.TryAdd(f) //parent not stored yet
		assert.False(t, stored)

		stored, new = queue.TryAdd(d)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 4, size)
		assert.Equal(t, uint64(2), height)

		stored, _ = queue.TryAdd(dBroken) // wrong height
		assert.False(t, stored)

		stored, new = queue.TryAdd(e)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 5, size)
		assert.Equal(t, uint64(3), height)

		stored, new = queue.TryAdd(f)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 6, size)
		assert.Equal(t, uint64(3), height)

		stored, new = queue.TryAdd(g)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, stored)
		assert.True(t, new)
		assert.Equal(t, 7, size)
		assert.Equal(t, uint64(3), height)
	})

	t.Run("Dismounting", func(t *testing.T) {
		// dismount queue
		blockA, queuesA := queue.Dismount()
		assert.Equal(t, a, blockA)
		require.Len(t, queuesA, 1)
		assert.Equal(t, 6, queuesA[0].Size())
		assert.Equal(t, uint64(2), queuesA[0].Height())

		blockC, queuesC := queuesA[0].Dismount()
		assert.Equal(t, c, blockC)
		require.Len(t, queuesC, 2)

		// order of children is not guaranteed
		var queueD *Queue
		var queueB *Queue
		if queuesC[0].Head.Item == d {
			queueD = queuesC[0]
			queueB = queuesC[1]
		} else {
			queueD = queuesC[1]
			queueB = queuesC[0]
		}
		assert.Equal(t, d, queueD.Head.Item)
		sizeD := queueD.Size()
		heightD := queueD.Height()
		sizeB := queueB.Size()
		heightB := queueB.Height()

		assert.Equal(t, 3, sizeD)
		assert.Equal(t, uint64(1), heightD)
		assert.Equal(t, 2, sizeB)
		assert.Equal(t, uint64(1), heightB)

		blockD, queuesD := queueD.Dismount()
		assert.Equal(t, d, blockD)
		assert.Len(t, queuesD, 2)
	})

	t.Run("Process all", func(t *testing.T) {
		// Dismounting iteratively all queues should yield all nodes/blocks only once
		// and in the proper order (parents are always evaluated first)
		blocksInOrder := make([]*entity.ExecutableBlock, 0)

		executionHeads := make(chan *Queue, 10)
		executionHeads <- queue

		for len(executionHeads) > 0 {
			currentHead := <-executionHeads
			block, newQueues := currentHead.Dismount()
			blocksInOrder = append(blocksInOrder, block.(*entity.ExecutableBlock))
			for _, newQueue := range newQueues {
				executionHeads <- newQueue
			}
		}

		// Couldn't find ready assertion for subset in order, so lets
		// map nodes by their index and check if order is as expected
		indices := make(map[*entity.ExecutableBlock]int)

		for i, block := range blocksInOrder {
			indices[block] = i
		}

		// a -> c -> b -> g
		assert.Less(t, indices[a], indices[c])
		assert.Less(t, indices[c], indices[b])
		assert.Less(t, indices[b], indices[g])

		// a -> c -> d -> f
		assert.Less(t, indices[a], indices[c])
		assert.Less(t, indices[c], indices[d])
		assert.Less(t, indices[d], indices[f])

		// a -> c -> d -> e
		assert.Less(t, indices[a], indices[c])
		assert.Less(t, indices[c], indices[d])
		assert.Less(t, indices[d], indices[e])
	})

	//t.Run("Attaching", func(t *testing.T) {
	//	queue := NewQueue(a)
	//
	//	added, new := queue.TryAdd(c)
	//	assert.True(t, added)
	//	assert.True(t, new)
	//	assert.Equal(t, 2, queue.Size())
	//	assert.Equal(t, uint64(1), queue.Height())
	//
	//	queueB := NewQueue(b)
	//	added, new = queueB.TryAdd(g)
	//	assert.True(t, added)
	//	assert.True(t, new)
	//
	//	assert.Equal(t, 2, queueB.Size())
	//	assert.Equal(t, uint64(1), queueB.Height())
	//
	//	queueF := NewQueue(f)
	//
	//	err := queue.Attach(queueF) // node D is missing
	//	assert.Error(t, err)
	//
	//	err = queue.Attach(queueB)
	//	assert.NoError(t, err)
	//	assert.Equal(t, 4, queue.Size())
	//	assert.Equal(t, uint64(3), queue.Height())
	//
	//	added, new = queue.TryAdd(d)
	//	assert.True(t, added)
	//	assert.True(t, new)
	//	assert.Equal(t, 5, queue.Size())
	//	assert.Equal(t, uint64(3), queue.Height())
	//
	//	err = queue.Attach(queueF) // node D is now in the queue
	//	assert.NoError(t, err)
	//	assert.Equal(t, 6, queue.Size())
	//	assert.Equal(t, uint64(3), queue.Height())
	//})

	// Creating queue:
	//    f--d--c-a
	// Addingan element should be an idempotent operation:
	//   * adding c a second time
	//   * Dequeueing single head:
	//     we should only get one child queue f--d--c
	t.Run("Adding_Idempotent", func(t *testing.T) {
		queue := NewQueue(a)
		add, new := queue.TryAdd(c)
		assert.True(t, add)
		assert.True(t, new)

		add, new = queue.TryAdd(d)
		assert.True(t, add)
		assert.True(t, new)

		add, new = queue.TryAdd(f)
		assert.True(t, add)
		assert.True(t, new)

		assert.Equal(t, 4, queue.Size())
		assert.Equal(t, uint64(3), queue.Height())

		// adding c a second time
		add, new = queue.TryAdd(c)
		assert.True(t, add)
		assert.False(t, new)

		// Dequeueing a
		head, childQueues := queue.Dismount()
		assert.Equal(t, a, head)
		assert.Equal(t, 1, len(childQueues), "There should only be a single child queue")
		assert.Equal(t, c.ID(), childQueues[0].Head.Item.ID())
	})

	// Testing attaching overlapping queues:
	// queue A:
	//   g-b
	//      \
	//       c
	// queue B:
	//    d--c-a
	// attach queueA to queueB: we expect an error as the queues have nodes in common
	//t.Run("Attaching_partially_overlapped_queue", func(t *testing.T) {
	//	queueA := NewQueue(c)
	//	add, new := queueA.TryAdd(b)
	//	assert.True(t, add)
	//	assert.True(t, new)
	//
	//	add, new = queueA.TryAdd(g)
	//	assert.True(t, add)
	//	assert.True(t, new)
	//
	//	queueB := NewQueue(a)
	//	add, new = queueB.TryAdd(c)
	//	assert.True(t, add)
	//	assert.True(t, new)
	//
	//	add, new = queueB.TryAdd(d)
	//	assert.True(t, add)
	//	assert.True(t, new)
	//
	//	err := queueB.Attach(queueA)
	//	assert.Error(t, err)
	//})

}
