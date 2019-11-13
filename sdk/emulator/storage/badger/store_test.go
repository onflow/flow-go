package badger_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"

	"github.com/dapperlabs/flow-go/sdk/emulator/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlocks(t *testing.T) {
	store, dir := setupStore(t)
	defer require.Nil(t, os.RemoveAll(dir))

	block1 := types.Block{
		Number: 1,
	}
	block2 := types.Block{
		Number: 2,
	}

	t.Run("should return error for not found", func(t *testing.T) {
		t.Run("GetBlockByHash", func(t *testing.T) {
			_, err := store.GetBlockByHash(unittest.HashFixture(32))
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})

		t.Run("GetBlockByNumber", func(t *testing.T) {
			_, err := store.GetBlockByNumber(block1.Number)
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})

		t.Run("GetLatestBlock", func(t *testing.T) {
			_, err := store.GetLatestBlock()
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})
	})

	t.Run("should be able to insert block", func(t *testing.T) {
		err := store.InsertBlock(block1)
		assert.Nil(t, err)
	})

	// insert block 1
	err := store.InsertBlock(block1)
	assert.Nil(t, err)

	t.Run("should be able to get inserted block", func(t *testing.T) {
		t.Run("GetBlockByNumber", func(t *testing.T) {
			block, err := store.GetBlockByNumber(block1.Number)
			assert.Nil(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("GetBlockByHash", func(t *testing.T) {
			block, err := store.GetBlockByHash(block1.Hash())
			assert.Nil(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("GetLatestBlock", func(t *testing.T) {
			block, err := store.GetLatestBlock()
			assert.Nil(t, err)
			assert.Equal(t, block1, block)
		})
	})

	// insert block 2
	err = store.InsertBlock(block2)
	assert.Nil(t, err)

	t.Run("Latest block should update", func(t *testing.T) {
		block, err := store.GetLatestBlock()
		assert.Nil(t, err)
		assert.Equal(t, block2, block)
	})
}

func TestTransactions(t *testing.T) {
	store, dir := setupStore(t)
	defer require.Nil(t, os.RemoveAll(dir))

	tx := unittest.TransactionFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.GetTransaction(tx.Hash())
		if assert.Error(t, err) {
			assert.IsType(t, storage.ErrNotFound{}, err)
		}
	})

	t.Run("should be able to insert tx", func(t *testing.T) {
		err := store.InsertTransaction(tx)
		assert.Nil(t, err)

		t.Run("should be able to get inserted tx", func(t *testing.T) {
			storedTx, err := store.GetTransaction(tx.Hash())
			require.Nil(t, err)
			assert.Equal(t, tx, storedTx)
		})
	})
}

func TestRegisters(t *testing.T) {
	store, dir := setupStore(t)
	defer require.Nil(t, os.RemoveAll(dir))

	var blockNumber uint64 = 1
	registers := unittest.RegistersFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.GetRegistersView(blockNumber)
		if assert.Error(t, err) {
			assert.IsType(t, storage.ErrNotFound{}, err)
		}
	})

	t.Run("should be able set registers", func(t *testing.T) {
		err := store.SetRegisters(blockNumber, registers)
		assert.Nil(t, err)

		t.Run("Should be to get set registers", func(t *testing.T) {
			view, err := store.GetRegistersView(blockNumber)
			assert.Nil(t, err)
			assert.Equal(t, registers.NewView(), &view)
		})
	})
}

func TestEvents(t *testing.T) {
	store, dir := setupStore(t)
	defer require.Nil(t, os.RemoveAll(dir))

	t.Run("should be able to insert events", func(t *testing.T) {
		events := []flow.Event{unittest.EventFixture()}
		var blockNumber uint64 = 1

		err := store.InsertEvents(blockNumber, events...)
		assert.Nil(t, err)

		t.Run("should be able to get inserted events", func(t *testing.T) {
			gotEvents, err := store.GetEvents("", blockNumber, blockNumber)
			assert.Nil(t, err)
			assert.Equal(t, events, gotEvents)
		})
	})

	t.Run("should be able to insert many events", func(t *testing.T) {
		// block 1 will have 1 event type=1
		// block 2 will have 2 events, types=1,2
		// and so on...
		eventsByBlock := make(map[uint64][]flow.Event)
		for i := 1; i <= 10; i++ {
			var events []flow.Event
			for j := 1; j <= i; j++ {
				event := unittest.EventFixture()
				event.Type = fmt.Sprintf("%d", j)
				events = append(events, event)
			}
			eventsByBlock[uint64(i)] = events
			err := store.InsertEvents(uint64(i), events...)
			assert.Nil(t, err)
		}

		t.Run("should be able to query by block", func(t *testing.T) {
			t.Run("block 1", func(t *testing.T) {
				gotEvents, err := store.GetEvents("", 1, 1)
				assert.Nil(t, err)
				assert.Equal(t, eventsByBlock[1], gotEvents)
			})

			t.Run("block 2", func(t *testing.T) {
				gotEvents, err := store.GetEvents("", 2, 2)
				assert.Nil(t, err)
				assert.Equal(t, eventsByBlock[2], gotEvents)
			})
		})

		t.Run("should be able to query by block interval", func(t *testing.T) {
			t.Run("block 1->2", func(t *testing.T) {
				gotEvents, err := store.GetEvents("", 1, 2)
				assert.Nil(t, err)
				assert.Equal(t, append(eventsByBlock[1], eventsByBlock[2]...), gotEvents)
			})

			t.Run("block 5->10", func(t *testing.T) {
				gotEvents, err := store.GetEvents("", 5, 10)
				assert.Nil(t, err)

				var expectedEvents []flow.Event
				for i := 5; i <= 10; i++ {
					expectedEvents = append(expectedEvents, eventsByBlock[uint64(i)]...)
				}
				assert.Equal(t, expectedEvents, gotEvents)
			})
		})

		t.Run("should be able to query by event type", func(t *testing.T) {
			t.Run("type=1, block=1", func(t *testing.T) {
				// should be one event type=1 in block 1
				gotEvents, err := store.GetEvents("1", 1, 1)
				assert.Nil(t, err)
				assert.Len(t, gotEvents, 1)
				assert.Equal(t, "1", gotEvents[0].Type)
			})

			t.Run("type=1, block=1->10", func(t *testing.T) {
				// should be 10 events type=1 in blocks 1->10
				gotEvents, err := store.GetEvents("1", 1, 10)
				assert.Nil(t, err)
				assert.Len(t, gotEvents, 10)
				for _, event := range gotEvents {
					assert.Equal(t, "1", event.Type)
				}
			})

			t.Run("type=2, block=1", func(t *testing.T) {
				// should be 0 type=2 events here
				gotEvents, err := store.GetEvents("2", 1, 1)
				assert.Nil(t, err)
				assert.Len(t, gotEvents, 0)
			})
		})
	})
}

// setStore creates a temporary directory for the Badger and creates a
// badger.Store instance. The caller is responsible for closing the store
// and deleting the temporary directory.
func setupStore(t *testing.T) (storage.Store, string) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)

	store, err := badger.New(&badger.Config{Path: dir})
	require.Nil(t, err)

	return store, dir
}
