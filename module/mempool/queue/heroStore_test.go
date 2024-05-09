package queue_test

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeroStore_Sequential(t *testing.T) {
	sizeLimit := 100

	// sequentially put and get
	store := queue.NewHeroStore(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	messages := unittest.EngineMessageFixtures(sizeLimit)
	for i := 0; i < sizeLimit; i++ {
		require.True(t, store.Put(messages[i]))

		// duplicate put should fail
		require.False(t, store.Put(messages[i]))
	}

	// once store meets the size limit, any extra put should fail.
	for i := 0; i < 100; i++ {
		require.False(t, store.Put(unittest.EngineMessageFixture()))
	}

	// getting entities sequentially.
	for i := 0; i < sizeLimit; i++ {
		msg, ok := store.Get()
		require.True(t, ok)
		require.Equal(t, messages[i], msg)
	}
}

// TestHeroStore_Concurrent evaluates correctness of store implementation against concurrent put and get.
func TestHeroStore_Concurrent(t *testing.T) {
	sizeLimit := 100
	store := queue.NewHeroStore(uint32(sizeLimit), unittest.Logger(), metrics.NewNoopCollector())

	// initially there should be nothing to pop
	msg, ok := store.Get()
	require.False(t, ok)
	require.Nil(t, msg)

	putWG := &sync.WaitGroup{}
	putWG.Add(sizeLimit)

	messages := unittest.EngineMessageFixtures(sizeLimit)
	// putting messages concurrently.
	for _, m := range messages {
		m := m
		go func() {
			require.True(t, store.Put(m))
			putWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, putWG.Wait, 100*time.Millisecond, "could not put all messages on time")

	// once store meets the size limit, any extra put should fail.
	putWG.Add(sizeLimit)
	for i := 0; i < sizeLimit; i++ {
		go func() {
			require.False(t, store.Put(unittest.EngineMessageFixture()))
			putWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, putWG.Wait, 100*time.Millisecond, "could not put all messages on time")

	getWG := &sync.WaitGroup{}
	getWG.Add(sizeLimit)
	matchLock := &sync.Mutex{}

	// pop-ing entities concurrently.
	for i := 0; i < sizeLimit; i++ {
		go func() {
			msg, ok := store.Get()
			require.True(t, ok)

			matchLock.Lock()
			matchAndRemoveMessages(t, messages, msg)
			matchLock.Unlock()

			getWG.Done()
		}()
	}
	unittest.RequireReturnsBefore(t, getWG.Wait, 100*time.Millisecond, "could not get all messages on time")

	// store must be empty after getting all
	msg, ok = store.Get()
	require.False(t, ok)
	require.Nil(t, msg)
}

// matchAndRemove checks existence of the msg in the "messages" array, and if a match is found, it is removed.
// If no match is found for a message, it fails the test.
func matchAndRemoveMessages(t *testing.T, messages []*engine.Message, msg *engine.Message) []*engine.Message {
	for i, m := range messages {
		if m.OriginID == msg.OriginID {
			require.Equal(t, m, msg)
			// removes the matched message from the list
			messages = append(messages[:i], messages[i+1:]...)
			return messages
		}
	}

	// no message found in the list to match
	require.Fail(t, "could not find a match for the message")
	return nil
}
