package badger_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"

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
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	block1 := types.Block{
		Number: 1,
	}
	block2 := types.Block{
		Number: 2,
	}

	t.Run("should return error for not found", func(t *testing.T) {
		t.Run("BlockByHash", func(t *testing.T) {
			_, err := store.BlockByHash(unittest.HashFixture(32))
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})

		t.Run("BlockByNumber", func(t *testing.T) {
			_, err := store.BlockByNumber(block1.Number)
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})

		t.Run("LatestBlock", func(t *testing.T) {
			_, err := store.LatestBlock()
			if assert.Error(t, err) {
				assert.IsType(t, storage.ErrNotFound{}, err)
			}
		})
	})

	t.Run("should be able to insert block", func(t *testing.T) {
		err := store.InsertBlock(block1)
		assert.NoError(t, err)
	})

	// insert block 1
	err := store.InsertBlock(block1)
	assert.NoError(t, err)

	t.Run("should be able to get inserted block", func(t *testing.T) {
		t.Run("BlockByNumber", func(t *testing.T) {
			block, err := store.BlockByNumber(block1.Number)
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("BlockByHash", func(t *testing.T) {
			block, err := store.BlockByHash(block1.Hash())
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})

		t.Run("LatestBlock", func(t *testing.T) {
			block, err := store.LatestBlock()
			assert.NoError(t, err)
			assert.Equal(t, block1, block)
		})
	})

	// insert block 2
	err = store.InsertBlock(block2)
	assert.NoError(t, err)

	t.Run("Latest block should update", func(t *testing.T) {
		block, err := store.LatestBlock()
		assert.NoError(t, err)
		assert.Equal(t, block2, block)
	})
}

func TestTransactions(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	tx := unittest.TransactionFixture()

	t.Run("should return error for not found", func(t *testing.T) {
		_, err := store.TransactionByHash(tx.Hash())
		if assert.Error(t, err) {
			assert.IsType(t, storage.ErrNotFound{}, err)
		}
	})

	t.Run("should be able to insert tx", func(t *testing.T) {
		err := store.InsertTransaction(tx)
		assert.NoError(t, err)

		t.Run("should be able to get inserted tx", func(t *testing.T) {
			storedTx, err := store.TransactionByHash(tx.Hash())
			require.Nil(t, err)
			assert.Equal(t, tx, storedTx)
		})
	})
}

func TestLedger(t *testing.T) {
	t.Run("get/set", func(t *testing.T) {
		store, dir := setupStore(t)
		defer func() {
			require.Nil(t, store.Close())
			require.Nil(t, os.RemoveAll(dir))
		}()

		var blockNumber uint64 = 1

		ledger := make(types.LedgerDelta)
		ledger["foo"] = []byte("bar")

		t.Run("should get able to set ledger", func(t *testing.T) {
			err := store.InsertLedgerDelta(blockNumber, ledger)
			assert.NoError(t, err)
		})

		t.Run("should be to get set ledger", func(t *testing.T) {
			gotLedger := store.LedgerViewByNumber(blockNumber)
			gotRegister, err := gotLedger.Get("foo")
			assert.NoError(t, err)
			assert.Equal(t, ledger["foo"], gotRegister)
		})
	})

	t.Run("versioning", func(t *testing.T) {
		store, dir := setupStore(t)
		defer func() {
			require.Nil(t, store.Close())
			require.Nil(t, os.RemoveAll(dir))
		}()

		// Create a list of ledgers, where the ledger at index i has
		// keys (i+2)-1->(i+2)+1 set to value i-1.
		totalBlocks := 10
		var ledgers []types.LedgerDelta
		for i := 2; i < totalBlocks+2; i++ {
			ledger := make(types.LedgerDelta)
			for j := i - 1; j <= i+1; j++ {
				ledger[fmt.Sprintf("%d", j)] = []byte{byte(i - 1)}
			}
			ledgers = append(ledgers, ledger)
		}
		require.Equal(t, totalBlocks, len(ledgers))

		// Insert all the ledgers, starting with block 1.
		// This will result in a ledger state that looks like this:
		// Block 1: {1: 1, 2: 1, 3: 1}
		// Block 2: {2: 2, 3: 2, 4: 2}
		// ...
		// The combined state at block N looks like:
		// {1: 1, 2: 2, 3: 3, ..., N+1: N, N+2: N}
		for i, ledger := range ledgers {
			err := store.InsertLedgerDelta(uint64(i+1), ledger)
			require.NoError(t, err)
		}

		// View at block 1 should have keys 1, 2, 3
		t.Run("should version the first written block", func(t *testing.T) {
			gotLedger := store.LedgerViewByNumber(1)
			for i := 1; i <= 3; i++ {
				val, err := gotLedger.Get(fmt.Sprintf("%d", i))
				assert.NoError(t, err)
				assert.Equal(t, []byte{byte(1)}, val)
			}
		})

		// View at block N should have values 1->N+2
		t.Run("should version all blocks", func(t *testing.T) {
			for block := 2; block < totalBlocks; block++ {
				gotLedger := store.LedgerViewByNumber(uint64(block))
				// The keys 1->N-1 are defined in previous blocks
				for i := 1; i < block; i++ {
					val, err := gotLedger.Get(fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(i)}, val)
				}
				// The keys N->N+2 are defined in the queried block
				for i := block; i <= block+2; i++ {
					val, err := gotLedger.Get(fmt.Sprintf("%d", i))
					assert.NoError(t, err)
					assert.Equal(t, []byte{byte(block)}, val)
				}
			}
		})
	})
}

func TestEvents(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	t.Run("should be able to insert events", func(t *testing.T) {
		events := []flow.Event{unittest.EventFixture(func(e *flow.Event) {
			e.Payload = []byte{1, 2, 3, 4}
		})}
		var blockNumber uint64 = 1

		err := store.InsertEvents(blockNumber, events)
		assert.NoError(t, err)

		t.Run("should be able to get inserted events", func(t *testing.T) {
			gotEvents, err := store.RetrieveEvents("", blockNumber, blockNumber)
			assert.NoError(t, err)
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
				event := unittest.EventFixture(func(e *flow.Event) {
					e.Payload = []byte{1, 2, 3, 4}
					e.Type = fmt.Sprintf("%d", j)
				})
				events = append(events, event)
			}
			eventsByBlock[uint64(i)] = events
			err := store.InsertEvents(uint64(i), events)
			assert.NoError(t, err)
		}

		t.Run("should be able to query by block", func(t *testing.T) {
			t.Run("block 1", func(t *testing.T) {
				gotEvents, err := store.RetrieveEvents("", 1, 1)
				assert.NoError(t, err)
				assert.Equal(t, eventsByBlock[1], gotEvents)
			})

			t.Run("block 2", func(t *testing.T) {
				gotEvents, err := store.RetrieveEvents("", 2, 2)
				assert.NoError(t, err)
				assert.Equal(t, eventsByBlock[2], gotEvents)
			})
		})

		t.Run("should be able to query by block interval", func(t *testing.T) {
			t.Run("block 1->2", func(t *testing.T) {
				gotEvents, err := store.RetrieveEvents("", 1, 2)
				assert.NoError(t, err)
				assert.Equal(t, append(eventsByBlock[1], eventsByBlock[2]...), gotEvents)
			})

			t.Run("block 5->10", func(t *testing.T) {
				gotEvents, err := store.RetrieveEvents("", 5, 10)
				assert.NoError(t, err)

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
				gotEvents, err := store.RetrieveEvents("1", 1, 1)
				assert.NoError(t, err)
				assert.Len(t, gotEvents, 1)
				assert.Equal(t, "1", gotEvents[0].Type)
			})

			t.Run("type=1, block=1->10", func(t *testing.T) {
				// should be 10 events type=1 in Blocks 1->10
				gotEvents, err := store.RetrieveEvents("1", 1, 10)
				assert.NoError(t, err)
				assert.Len(t, gotEvents, 10)
				for _, event := range gotEvents {
					assert.Equal(t, "1", event.Type)
				}
			})

			t.Run("type=2, block=1", func(t *testing.T) {
				// should be 0 type=2 events here
				gotEvents, err := store.RetrieveEvents("2", 1, 1)
				assert.NoError(t, err)
				assert.Len(t, gotEvents, 0)
			})
		})
	})
}

func TestPersistence(t *testing.T) {
	store, dir := setupStore(t)
	defer func() {
		require.Nil(t, store.Close())
		require.Nil(t, os.RemoveAll(dir))
	}()

	block := types.Block{Number: 1}
	tx := unittest.TransactionFixture()
	events := []flow.Event{unittest.EventFixture(func(e *flow.Event) {
		e.Payload = []byte{1, 2, 3, 4}
	})}

	ledger := make(types.LedgerDelta)
	ledger["foo"] = []byte("bar")

	// insert some stuff to to the store
	err := store.InsertBlock(block)
	assert.NoError(t, err)
	err = store.InsertTransaction(tx)
	assert.NoError(t, err)
	err = store.InsertEvents(block.Number, events)
	assert.NoError(t, err)
	err = store.InsertLedgerDelta(block.Number, ledger)

	// close the store
	err = store.Close()
	assert.NoError(t, err)

	// create a new store with the same database directory
	store, err = badger.New(badger.WithPath(dir))
	require.Nil(t, err)

	// should be able to retrieve what we stored
	gotBlock, err := store.LatestBlock()
	assert.NoError(t, err)
	assert.Equal(t, block, gotBlock)

	gotTx, err := store.TransactionByHash(tx.Hash())
	assert.NoError(t, err)
	assert.Equal(t, tx, gotTx)

	gotEvents, err := store.RetrieveEvents("", block.Number, block.Number)
	assert.NoError(t, err)
	assert.Equal(t, events, gotEvents)

	gotLedger := store.LedgerViewByNumber(block.Number)
	gotRegister, err := gotLedger.Get("foo")
	assert.NoError(t, err)
	assert.Equal(t, ledger["foo"], gotRegister)
}

func benchmarkInsertLedgerDelta(b *testing.B, nKeys int) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	ledger := make(types.LedgerDelta)
	for i := 0; i < nKeys; i++ {
		ledger[fmt.Sprintf("%d", i)] = []byte{byte(i)}
	}

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		if err := store.InsertLedgerDelta(1, ledger); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkInsertLedgerDelta1(b *testing.B)    { benchmarkInsertLedgerDelta(b, 1) }
func BenchmarkInsertLedgerDelta10(b *testing.B)   { benchmarkInsertLedgerDelta(b, 10) }
func BenchmarkInsertLedgerDelta100(b *testing.B)  { benchmarkInsertLedgerDelta(b, 100) }
func BenchmarkInsertLedgerDelta1000(b *testing.B) { benchmarkInsertLedgerDelta(b, 1000) }

func BenchmarkBlockDiskUsage(b *testing.B) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.StartTimer()
	var lastDBSize int64
	for i := 0; i < b.N; i++ {
		block := types.Block{
			Number:            uint64(i),
			PreviousBlockHash: unittest.HashFixture(32),
			TransactionHashes: []crypto.Hash{unittest.HashFixture(32)},
		}
		if err := store.InsertBlock(block); err != nil {
			b.Fatal(err)
		}
		if err := store.Sync(); err != nil {
			b.Fatal(err)
		}

		size, err := dirSize(dir)
		if err != nil {
			b.Fatal(err)
		}

		dbSizeIncrease := size - lastDBSize
		b.ReportMetric(float64(dbSizeIncrease), "db_size_increase_bytes/op")
		lastDBSize = size
	}
}

func BenchmarkLedgerDiskUsage(b *testing.B) {
	b.StopTimer()
	dir, err := ioutil.TempDir("", "badger-test")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(dir)
	store, err := badger.New(badger.WithPath(dir))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()

	b.StartTimer()
	var lastDBSize int64
	for i := 0; i < b.N; i++ {
		ledger := make(types.LedgerDelta)
		for j := 0; j < 100; j++ {
			ledger[fmt.Sprintf("%d-%d", i, j)] = []byte{byte(i), byte(j)}
		}
		if err := store.InsertLedgerDelta(uint64(i), ledger); err != nil {
			b.Fatal(err)
		}
		if err := store.Sync(); err != nil {
			b.Fatal(err)
		}

		size, err := dirSize(dir)
		if err != nil {
			b.Fatal(err)
		}

		dbSizeIncrease := size - lastDBSize
		b.ReportMetric(float64(dbSizeIncrease), "db_size_increase_bytes/op")
		lastDBSize = size
	}
}

// setupStore creates a temporary directory for the Badger and creates a
// badger.Store instance. The caller is responsible for closing the store
// and deleting the temporary directory.
func setupStore(t *testing.T) (*badger.Store, string) {
	dir, err := ioutil.TempDir("", "badger-test")
	require.Nil(t, err)

	store, err := badger.New(badger.WithPath(dir))
	require.Nil(t, err)

	return store, dir
}

// Returns the size of a directory and all contents
func dirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return size, err
}
