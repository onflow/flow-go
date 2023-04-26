package synchronizer_test

import (
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/execution/state/storehouse/queue"
	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/ephemeral"
	"github.com/onflow/flow-go/engine/execution/state/storehouse/storage/forest"
	"github.com/onflow/flow-go/engine/execution/state/storehouse/synchronizer"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestSynchronizer(t *testing.T) {

	t.Run("happy path", func(t *testing.T) {

		blocks := unittest.BlockHeaderFixtures(10)
		genesis, headers := blocks[0], blocks[1:]

		syncFreq := 50 * time.Millisecond

		storage, err := forest.NewStorage(ephemeral.NewStorage(100, genesis, nil))
		require.NoError(t, err)

		blockQueue := queue.NewFinalizedBlockQueue(genesis)

		c, err := synchronizer.NewSynchronizer(storage, blockQueue, syncFreq)
		require.NoError(t, err)

		<-c.Ready()
		defer func() {
			<-c.Done()
		}()

		wg := sync.WaitGroup{}

		// block execution
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, h := range headers {
				err = c.BlockExecuted(h, nil)
				require.NoError(t, err)

				time.Sleep(time.Duration(rand.Intn(4)+1) * 50 * time.Millisecond)
			}
		}()

		// block finalizer
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, h := range headers {
				err = c.BlockFinalized(h)
				require.NoError(t, err)

				time.Sleep(100 * time.Millisecond)
			}
		}()

		wg.Wait()

		require.Eventuallyf(t, func() bool {
			ret, err := storage.LastCommittedBlock()
			require.NoError(t, err)
			return headers[8] == ret
		}, 2*syncFreq, syncFreq/10, "final commit not matching")
	})

	t.Run("finalization falling behind", func(t *testing.T) {

		blocks := unittest.BlockHeaderFixtures(10)
		genesis, headers := blocks[0], blocks[1:]

		storage, err := forest.NewStorage(ephemeral.NewStorage(100, genesis, nil))
		require.NoError(t, err)

		blockQueue := queue.NewFinalizedBlockQueue(genesis)

		c, err := synchronizer.NewSynchronizer(storage, blockQueue, 0)
		require.NoError(t, err)

		<-c.Ready()
		defer func() {
			<-c.Done()
		}()

		for _, h := range headers {
			err = c.BlockExecuted(h, nil)
			require.NoError(t, err)
		}

		for _, h := range headers {
			err = c.BlockFinalized(h)
			require.NoError(t, err)

			err = c.TrySync()
			require.NoError(t, err)

			ret, err := storage.LastCommittedBlock()
			require.NoError(t, err)
			require.Equal(t, h, ret)
		}
	})

	t.Run("execution falling behind", func(t *testing.T) {

		blocks := unittest.BlockHeaderFixtures(10)
		genesis, headers := blocks[0], blocks[1:]

		storage, err := forest.NewStorage(ephemeral.NewStorage(100, genesis, nil))
		require.NoError(t, err)

		blockQueue := queue.NewFinalizedBlockQueue(genesis)

		c, err := synchronizer.NewSynchronizer(storage, blockQueue, 0)
		require.NoError(t, err)

		<-c.Ready()
		defer func() {
			<-c.Done()
		}()

		for _, h := range headers {
			err = c.BlockFinalized(h)
			require.NoError(t, err)
		}

		for _, h := range headers {
			err = c.BlockExecuted(h, nil)
			require.NoError(t, err)

			err = c.TrySync()
			require.NoError(t, err)

			ret, err := storage.LastCommittedBlock()
			require.NoError(t, err)
			require.Equal(t, h, ret)
		}
	})

	t.Run("sync with both queues", func(t *testing.T) {

		blocks := unittest.BlockHeaderFixtures(10)
		genesis, headers := blocks[0], blocks[1:]

		storage, err := forest.NewStorage(ephemeral.NewStorage(100, genesis, nil))
		require.NoError(t, err)

		blockQueue := queue.NewFinalizedBlockQueue(genesis)

		c, err := synchronizer.NewSynchronizer(storage, blockQueue, 0)
		require.NoError(t, err)

		<-c.Ready()
		defer func() {
			<-c.Done()
		}()

		for _, h := range headers {
			err = c.BlockExecuted(h, nil)
			require.NoError(t, err)

			err = c.BlockFinalized(h)
			require.NoError(t, err)
		}

		err = c.TrySync()
		require.NoError(t, err)

		ret, err := storage.LastCommittedBlock()
		require.NoError(t, err)
		require.Equal(t, blocks[9], ret)
	})

}
