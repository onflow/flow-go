package indexes

import (
	"errors"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// testIndexStateLock reuses a registered lock name for IndexState unit tests.
// The test keys below use a unique prefix (0xFD) that does not collide with any
// concrete index type, so tests can share the same DB without interference.
const testIndexStateLock = storage.LockIndexAccountTransactions

var (
	testIndexStateLowerKey = []byte{0xFD, 0x01}
	testIndexStateUpperKey = []byte{0xFD, 0x02}
)

// bootstrapTestIndexState bootstraps an IndexState in a committed batch and returns it.
func bootstrapTestIndexState(tb testing.TB, db storage.DB, lm storage.LockManager, height uint64) *IndexState {
	tb.Helper()
	var state *IndexState
	err := unittest.WithLock(tb, lm, testIndexStateLock, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			var bootstrapErr error
			state, bootstrapErr = BootstrapIndexState(lctx, rw, db, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey, height)
			return bootstrapErr
		})
	})
	require.NoError(tb, err)
	return state
}

func TestNewIndexState(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized DB returns ErrNotBootstrapped", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			index, err := NewIndexState(storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
			require.Nil(t, index)
		})
	})

	t.Run("lower bound key present but upper bound missing returns exception", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			// Write only the lower bound, simulating partial / corrupted state.
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), testIndexStateLowerKey, uint64(5))
			})
			require.NoError(t, err)

			index, err := NewIndexState(storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey)
			require.Nil(t, index)
			assert.False(t, errors.Is(err, storage.ErrNotBootstrapped), "partial state should not return ErrNotBootstrapped")
			require.Error(t, err)
		})
	})

	t.Run("fully bootstrapped DB returns correct heights", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			bootstrapTestIndexState(t, storageDB, lm, 42)

			state, err := NewIndexState(storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey)
			require.NoError(t, err)
			assert.Equal(t, uint64(42), state.FirstIndexedHeight())
			assert.Equal(t, uint64(42), state.LatestIndexedHeight())
		})
	})

	t.Run("bootstrapped at height 0 returns correct heights", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			bootstrapTestIndexState(t, storageDB, lm, 0)

			state, err := NewIndexState(storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey)
			require.NoError(t, err)
			assert.Equal(t, uint64(0), state.FirstIndexedHeight())
			assert.Equal(t, uint64(0), state.LatestIndexedHeight())
		})
	})
}

func TestBootstrapIndexState(t *testing.T) {
	t.Parallel()

	t.Run("without lock returns error", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			lctx := lm.NewContext()
			defer lctx.Release()

			// lctx does not hold testIndexStateLock
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				_, bootstrapErr := BootstrapIndexState(lctx, rw, storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey, 1)
				return bootstrapErr
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})

	t.Run("clean DB bootstraps successfully", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			state := bootstrapTestIndexState(t, storageDB, lm, 10)
			assert.Equal(t, uint64(10), state.FirstIndexedHeight())
			assert.Equal(t, uint64(10), state.LatestIndexedHeight())
		})
	})

	t.Run("second bootstrap returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			bootstrapTestIndexState(t, storageDB, lm, 1)

			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					index, bootstrapErr := BootstrapIndexState(lctx, rw, storageDB, testIndexStateLock, testIndexStateLowerKey, testIndexStateUpperKey, 1)
					require.Nil(t, index)
					return bootstrapErr
				})
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("bootstrap at height 0", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			state := bootstrapTestIndexState(t, storageDB, lm, 0)
			assert.Equal(t, uint64(0), state.FirstIndexedHeight())
			assert.Equal(t, uint64(0), state.LatestIndexedHeight())
		})
	})
}

func TestIndexState_PrepareStore(t *testing.T) {
	t.Parallel()

	t.Run("without lock returns error", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			// lctx does not hold testIndexStateLock
			lctx := lm.NewContext()
			defer lctx.Release()

			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return state.PrepareStore(lctx, rw, 2)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})

	t.Run("consecutive height succeeds and advances latest height", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 2)
				})
			})
			require.NoError(t, err)
			assert.Equal(t, uint64(2), state.LatestIndexedHeight())
		})
	})

	t.Run("re-store at latest height returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 1)
				})
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("store below latest height returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 5)

			// advance to 6
			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 6)
				})
			})
			require.NoError(t, err)

			// try to store at 4 (below latest=6)
			err = unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 4)
				})
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("non-consecutive height returns error, not ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			// gap: latest=1, expected=2, got=5
			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 5)
				})
			})
			require.Error(t, err)
			assert.False(t, errors.Is(err, storage.ErrAlreadyExists),
				"non-consecutive height should not return ErrAlreadyExists")
		})
	})

	t.Run("in-memory height not updated when batch is not committed", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			require.Equal(t, uint64(1), state.LatestIndexedHeight())

			batch := state.db.NewBatch()
			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return state.PrepareStore(lctx, batch, 2)
			})
			require.NoError(t, err)

			// Close the batch without committing — discards the write.
			require.NoError(t, batch.Close())

			assert.Equal(t, uint64(1), state.LatestIndexedHeight(),
				"latestHeight must not update when batch is not committed")
		})
	})
}

func TestIndexState_Accessors(t *testing.T) {
	t.Parallel()

	t.Run("FirstIndexedHeight returns bootstrap height", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 77)
			assert.Equal(t, uint64(77), state.FirstIndexedHeight())
		})
	})

	t.Run("FirstIndexedHeight does not change after PrepareStore", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 10)

			err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return state.PrepareStore(lctx, rw, 11)
				})
			})
			require.NoError(t, err)

			assert.Equal(t, uint64(10), state.FirstIndexedHeight(),
				"FirstIndexedHeight must not change after PrepareStore")
		})
	})

	t.Run("LatestIndexedHeight advances with each committed PrepareStore", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()
			state := bootstrapTestIndexState(t, storageDB, lm, 1)

			for height := uint64(2); height <= 5; height++ {
				err := unittest.WithLock(t, lm, testIndexStateLock, func(lctx lockctx.Context) error {
					return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
						return state.PrepareStore(lctx, rw, height)
					})
				})
				require.NoError(t, err)
				assert.Equal(t, height, state.LatestIndexedHeight())
			}
		})
	})
}
