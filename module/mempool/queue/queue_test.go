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
	c := unittest.ExecutableBlockFixtureWithParent(0, a.Block.ID())
	b := unittest.ExecutableBlockFixtureWithParent(0, c.Block.ID())
	d := unittest.ExecutableBlockFixtureWithParent(0, c.Block.ID())
	e := unittest.ExecutableBlockFixtureWithParent(0, d.Block.ID())
	f := unittest.ExecutableBlockFixtureWithParent(0, d.Block.ID())
	g := unittest.ExecutableBlockFixtureWithParent(0, b.Block.ID())

	queue := NewQueue(a)

	t.Run("Adding", func(t *testing.T) {
		added := queue.TryAdd(b) //parent not added yet
		assert.False(t, added)

		added = queue.TryAdd(c)
		assert.True(t, added)

		added = queue.TryAdd(b)
		assert.True(t, added)

		added = queue.TryAdd(f) //parent not added yet
		assert.False(t, added)

		added = queue.TryAdd(d)
		assert.True(t, added)

		added = queue.TryAdd(e)
		assert.True(t, added)

		added = queue.TryAdd(f)
		assert.True(t, added)

		added = queue.TryAdd(g)
		assert.True(t, added)

	})

	t.Run("Dismounting", func(t *testing.T) {
		// dismount queue
		blockA, queuesA := queue.Dismount()

		assert.Equal(t, a, blockA)
		require.Len(t, queuesA, 1)

		blockC, queuesC := queuesA[0].Dismount()
		assert.Equal(t, c, blockC)
		require.Len(t, queuesC, 2)

		// order of children is not guaranteed
		var queueD *Queue
		if queuesC[0].Head.ExecutableBlock == d {
			queueD = queuesC[0]
		} else {
			queueD = queuesC[1]
		}
		assert.Equal(t, d, queueD.Head.ExecutableBlock)

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
			blocksInOrder = append(blocksInOrder, block)
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
}
