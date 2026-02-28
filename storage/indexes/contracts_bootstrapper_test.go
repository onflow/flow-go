package indexes

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestContractDeploymentsBootstrapper_Constructor(t *testing.T) {
	t.Parallel()

	t.Run("nil store when not bootstrapped", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			b, err := NewContractDeploymentsBootstrapper(storageDB, 5)
			require.NoError(t, err)
			// Store is nil: read operations return ErrNotBootstrapped
			_, err = b.ByContractID("A.1234567890abcdef.Foo")
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("loads existing bootstrapped index", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 7, nil, func(db storage.DB, _ storage.LockManager, _ *ContractDeploymentsIndex) {
			b, err := NewContractDeploymentsBootstrapper(db, 7)
			require.NoError(t, err)

			first, err := b.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(7), first)

			latest, err := b.LatestIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(7), latest)
		})
	})
}

func TestContractDeploymentsBootstrapper_BeforeBootstrap(t *testing.T) {
	t.Parallel()

	// A bootstrapper created from an empty DB: all read methods return ErrNotBootstrapped.
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		storageDB := pebbleimpl.ToDB(db)
		b, err := NewContractDeploymentsBootstrapper(storageDB, 5)
		require.NoError(t, err)

		t.Run("FirstIndexedHeight returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.FirstIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("LatestIndexedHeight returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.LatestIndexedHeight()
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("ByContractID returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.ByContractID("A.1234567890abcdef.Foo")
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("DeploymentsByContractID returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.DeploymentsByContractID("A.1234567890abcdef.Foo", nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("ByAddress returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.ByAddress(unittest.RandomAddressFixture(), nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})

		t.Run("All returns ErrNotBootstrapped", func(t *testing.T) {
			_, err := b.All(nil)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})
}

func TestContractDeploymentsBootstrapper_UninitializedFirstHeight(t *testing.T) {
	t.Parallel()

	t.Run("returns (initialStartHeight, false) before bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			b, err := NewContractDeploymentsBootstrapper(storageDB, 42)
			require.NoError(t, err)

			h, initialized := b.UninitializedFirstHeight()
			assert.Equal(t, uint64(42), h)
			assert.False(t, initialized)
		})
	})

	t.Run("returns (firstHeight, true) after bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			b, err := NewContractDeploymentsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			// Bootstrap by calling Store at the initial height.
			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 10, nil)
				})
			})
			require.NoError(t, err)

			h, initialized := b.UninitializedFirstHeight()
			assert.Equal(t, uint64(10), h)
			assert.True(t, initialized)
		})
	})
}

func TestContractDeploymentsBootstrapper_Store(t *testing.T) {
	t.Parallel()

	t.Run("store at wrong height before bootstrap returns ErrNotBootstrapped", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			b, err := NewContractDeploymentsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			// Store at height 5 when initialStartHeight is 10
			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 5, nil)
				})
			})
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("store at correct height bootstraps index", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			b, err := NewContractDeploymentsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 10, nil)
				})
			})
			require.NoError(t, err)

			// After bootstrapping, read operations should succeed.
			first, err := b.FirstIndexedHeight()
			require.NoError(t, err)
			assert.Equal(t, uint64(10), first)
		})
	})

	t.Run("subsequent heights work normally after bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			b, err := NewContractDeploymentsBootstrapper(storageDB, 10)
			require.NoError(t, err)

			// Bootstrap at height 10
			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 10, nil)
				})
			})
			require.NoError(t, err)

			// Store at height 11 should work
			contractID := "A.1234567890abcdef.MyContract"
			d := makeDeployment(contractID, 11, 0, 0)
			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 11, []access.ContractDeployment{d})
				})
			})
			require.NoError(t, err)

			// Verify the deployment is queryable
			result, err := b.ByContractID(contractID)
			require.NoError(t, err)
			assert.Equal(t, uint64(11), result.BlockHeight)
		})
	})

	t.Run("store with initial deployments at bootstrap height", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			b, err := NewContractDeploymentsBootstrapper(storageDB, 5)
			require.NoError(t, err)

			contractID := "A.1234567890abcdef.MyContract"
			d := makeDeployment(contractID, 5, 0, 0)

			err = unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					return b.Store(lctx, rw, 5, []access.ContractDeployment{d})
				})
			})
			require.NoError(t, err)

			result, err := b.ByContractID(contractID)
			require.NoError(t, err)
			assert.Equal(t, contractID, result.ContractID)
			assert.Equal(t, uint64(5), result.BlockHeight)
		})
	})
}
