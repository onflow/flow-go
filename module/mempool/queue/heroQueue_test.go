package queue_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestHeroQueue_Sequential evaluates correctness of queue implementation against sequential push and pop.
func TestHeroQueue_Sequential(t *testing.T) {
	sizeLimit := 100
	q := queue.NewHeroQueue(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially queue must be zero
	require.Zero(t, q.Size())

	// initially there should be nothing to pop
	entity, ok := q.Pop()
	require.False(t, ok)
	require.Nil(t, entity)

	entities := unittest.MockEntityListFixture(sizeLimit)
	// pushing entities sequentially.
	for i, e := range entities {
		require.True(t, q.Push(*e))

		// duplicate push should fail
		require.False(t, q.Push(*e))

		require.Equal(t, q.Size(), uint(i+1))
	}

	// once queue meets the size limit, any extra push should fail.
	for i := 0; i < 100; i++ {
		require.False(t, q.Push(*unittest.MockEntityFixture()))

		// size should not change
		require.Equal(t, q.Size(), uint(sizeLimit))
	}

	// pop-ing entities sequentially.
	for i, e := range entities {
		popedE, ok := q.Pop()
		require.True(t, ok)
		require.Equal(t, *e, popedE)
		require.Equal(t, e.ID(), popedE.ID())

		require.Equal(t, q.Size(), uint(len(entities)-i-1))
	}
}

// TestHeroQueue_Concurrent evaluates correctness of queue implementation against concurrent push and pop.
func TestHeroQueue_Concurrent(t *testing.T) {
	sizeLimit := 100
	q := queue.NewHeroQueue(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())
	// initially queue must be zero
	require.Zero(t, q.Size())
	// initially there should be nothing to pop
	entity, ok := q.Pop()
	require.False(t, ok)
	require.Nil(t, entity)

	pushWG := &sync.WaitGroup{}
	pushWG.Add(sizeLimit)

	entities := unittest.MockEntityListFixture(sizeLimit)
	// pushing entities concurrently.
	for _, e := range entities {
		e := e // suppress loop variable
		go func() {
			require.True(t, q.Push(*e))
			pushWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, pushWG.Wait, 100*time.Millisecond, "could not push all entities on time")

	// once queue meets the size limit, any extra push should fail.
	pushWG.Add(sizeLimit)
	for i := 0; i < sizeLimit; i++ {
		go func() {
			require.False(t, q.Push(*unittest.MockEntityFixture()))
			pushWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, pushWG.Wait, 100*time.Millisecond, "could not push all entities on time")

	popWG := &sync.WaitGroup{}
	popWG.Add(sizeLimit)
	matchLock := &sync.Mutex{}

	// pop-ing entities concurrently.
	for i := 0; i < sizeLimit; i++ {
		go func() {
			popedE, ok := q.Pop()
			require.True(t, ok)

			matchLock.Lock()
			matchAndRemoveEntity(t, entities, popedE.(unittest.MockEntity))
			matchLock.Unlock()

			popWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, popWG.Wait, 100*time.Millisecond, "could not pop all entities on time")

	// queue must be empty after pop-ing all
	require.Zero(t, q.Size())
}

// matchAndRemove checks existence of the entity in the "entities" array, and if a match is found, it is removed.
// If no match is found for an entity, it fails the test.
func matchAndRemoveEntity(t *testing.T, entities []*unittest.MockEntity, entity unittest.MockEntity) []*unittest.MockEntity {
	for i, e := range entities {
		if *e == entity {
			// removes the matched entity from the list
			entities = append(entities[:i], entities[i+1:]...)
			return entities
		}
	}

	// no entity found in the list to match
	require.Fail(t, "could not find a match for an entity")
	return nil
}
