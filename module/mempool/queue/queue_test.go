package queue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestQueue(t *testing.T) {

	/* Input queue:
	  g-b
	     \
	f--d--c-a
	  /
	 e

	*/

	a := unittest.ExecutableBlockFixture(0)
	c := unittest.ExecutableBlockFixtureWithParent(0, &a.Block.Header)
	b := unittest.ExecutableBlockFixtureWithParent(0, &c.Block.Header)
	d := unittest.ExecutableBlockFixtureWithParent(0, &c.Block.Header)
	e := unittest.ExecutableBlockFixtureWithParent(0, &d.Block.Header)
	f := unittest.ExecutableBlockFixtureWithParent(0, &d.Block.Header)
	g := unittest.ExecutableBlockFixtureWithParent(0, &b.Block.Header)

	dBroken := unittest.ExecutableBlockFixtureWithParent(0, &c.Block.Header)
	dBroken.Block.Height += 2 //change height

	queue := NewQueue(a)

	t.Run("Adding", func(t *testing.T) {
		added := queue.TryAdd(b) //parent not added yet
		size := queue.Size()
		height := queue.Height()
		assert.False(t, added)
		assert.Equal(t, 1, size)
		assert.Equal(t, uint64(0), height)

		added = queue.TryAdd(c)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
		assert.Equal(t, 2, size)
		assert.Equal(t, uint64(1), height)

		added = queue.TryAdd(b)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
		assert.Equal(t, 3, size)
		assert.Equal(t, uint64(2), height)

		added = queue.TryAdd(f) //parent not added yet
		assert.False(t, added)

		added = queue.TryAdd(d)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
		assert.Equal(t, 4, size)
		assert.Equal(t, uint64(2), height)

		added = queue.TryAdd(dBroken) // wrong height
		assert.False(t, added)

		added = queue.TryAdd(e)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
		assert.Equal(t, 5, size)
		assert.Equal(t, uint64(3), height)

		added = queue.TryAdd(f)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
		assert.Equal(t, 6, size)
		assert.Equal(t, uint64(3), height)

		added = queue.TryAdd(g)
		size = queue.Size()
		height = queue.Height()
		assert.True(t, added)
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

	t.Run("Attaching", func(t *testing.T) {
		queue := NewQueue(a)

		added := queue.TryAdd(c)
		assert.True(t, added)
		assert.Equal(t, 2, queue.Size())
		assert.Equal(t, uint64(1), queue.Height())

		queueB := NewQueue(b)
		added = queueB.TryAdd(g)
		assert.True(t, added)

		assert.Equal(t, 2, queueB.Size())
		assert.Equal(t, uint64(1), queueB.Height())

		queueF := NewQueue(f)

		err := queue.Attach(queueF) // node D is missing
		assert.Error(t, err)

		err = queue.Attach(queueB)
		assert.NoError(t, err)
		assert.Equal(t, 4, queue.Size())
		assert.Equal(t, uint64(3), queue.Height())

		added = queue.TryAdd(d)
		assert.True(t, added)
		assert.Equal(t, 5, queue.Size())
		assert.Equal(t, uint64(3), queue.Height())

		err = queue.Attach(queueF) // node D is now in the queue
		assert.NoError(t, err)
		assert.Equal(t, 6, queue.Size())
		assert.Equal(t, uint64(3), queue.Height())
	})
}
