package indexes

import (
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes/iterator"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// RunWithBootstrappedContractDeploymentsIndex creates a fresh Pebble database and bootstraps
// it for contract deployment indexing at the given start height with the given initial
// deployments. The callback receives the shared storage DB, lock manager, and the index.
func RunWithBootstrappedContractDeploymentsIndex(
	tb testing.TB,
	startHeight uint64,
	deployments []access.ContractDeployment,
	f func(db storage.DB, lockManager storage.LockManager, idx *ContractDeploymentsIndex),
) {
	unittest.RunWithPebbleDB(tb, func(db *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		var idx *ContractDeploymentsIndex
		storageDB := pebbleimpl.ToDB(db)
		err := unittest.WithLock(tb, lockManager, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
			return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				var bootstrapErr error
				idx, bootstrapErr = BootstrapContractDeployments(lctx, rw, storageDB, startHeight, deployments)
				return bootstrapErr
			})
		})
		require.NoError(tb, err)
		f(storageDB, lockManager, idx)
	})
}

// storeContractDeployments stores a block of contract deployments at the given height using the
// provided index and lock manager.
func storeContractDeployments(
	tb testing.TB,
	lm storage.LockManager,
	idx *ContractDeploymentsIndex,
	height uint64,
	deployments []access.ContractDeployment,
) error {
	return unittest.WithLock(tb, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
		return idx.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return idx.Store(lctx, rw, height, deployments)
		})
	})
}

// collectContractDeployments is a test helper that creates a DeploymentsByContract iterator
// and collects results via CollectResults.
func collectContractDeployments(
	tb testing.TB,
	idx *ContractDeploymentsIndex,
	addr flow.Address,
	name string,
	limit uint32,
	cursor *access.ContractDeploymentsCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) ([]access.ContractDeployment, *access.ContractDeploymentsCursor) {
	tb.Helper()
	iter, err := idx.DeploymentsByContract(addr, name, cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter)
	require.NoError(tb, err)
	return collected, nextCursor
}

// collectAllContracts is a test helper that creates an All iterator and collects results
// via CollectResults.
func collectAllContracts(
	tb testing.TB,
	idx *ContractDeploymentsIndex,
	limit uint32,
	cursor *access.ContractDeploymentsCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) ([]access.ContractDeployment, *access.ContractDeploymentsCursor) {
	tb.Helper()
	iter, err := idx.All(cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter)
	require.NoError(tb, err)
	return collected, nextCursor
}

// collectContractsByAddress is a test helper that creates a ByAddress iterator and collects
// results via CollectResults.
func collectContractsByAddress(
	tb testing.TB,
	idx *ContractDeploymentsIndex,
	addr flow.Address,
	limit uint32,
	cursor *access.ContractDeploymentsCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) ([]access.ContractDeployment, *access.ContractDeploymentsCursor) {
	tb.Helper()
	iter, err := idx.ByAddress(addr, cursor)
	require.NoError(tb, err)
	collected, nextCursor, err := iterator.CollectResults(iter, limit, filter)
	require.NoError(tb, err)
	return collected, nextCursor
}

// assertDeployment asserts that actual matches all fields of expected.
func assertDeployment(tb testing.TB, expected, actual access.ContractDeployment) {
	tb.Helper()
	assert.Equal(tb, expected.Address, actual.Address)
	assert.Equal(tb, expected.ContractName, actual.ContractName)
	assert.Equal(tb, expected.BlockHeight, actual.BlockHeight)
	assert.Equal(tb, expected.TransactionID, actual.TransactionID)
	assert.Equal(tb, expected.TransactionIndex, actual.TransactionIndex)
	assert.Equal(tb, expected.EventIndex, actual.EventIndex)
	assert.Equal(tb, expected.Code, actual.Code)
	assert.Equal(tb, expected.CodeHash, actual.CodeHash)
	assert.Equal(tb, expected.IsDeleted, actual.IsDeleted)
	assert.Equal(tb, expected.IsPlaceholder, actual.IsPlaceholder)
}

// makeDeployment builds a minimal access.ContractDeployment for use in tests.
func makeDeployment(addr flow.Address, name string, height uint64, txIndex, eventIndex uint32) access.ContractDeployment {
	fakeHash := unittest.IdentifierFixture()
	return access.ContractDeployment{
		ContractName:     name,
		Address:          addr,
		BlockHeight:      height,
		TransactionID:    unittest.IdentifierFixture(),
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Code:             []byte("access(all) contract MyContract {}"),
		CodeHash:         fakeHash[:],
	}
}

// ----------------------------------------------------------------------------
// NewContractDeploymentsIndex
// ----------------------------------------------------------------------------

func TestContractDeployments_NewIndex(t *testing.T) {
	t.Parallel()

	t.Run("uninitialized database returns ErrNotBootstrapped", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			_, err := NewContractDeploymentsIndex(storageDB)
			require.ErrorIs(t, err, storage.ErrNotBootstrapped)
		})
	})

	t.Run("corrupted DB with only first height key returns exception", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)

			// Write only the firstHeight key, simulating a corrupted state.
			err := storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.UpsertByKey(rw.Writer(), keyContractDeploymentFirstHeightKey, uint64(10))
			})
			require.NoError(t, err)

			_, err = NewContractDeploymentsIndex(storageDB)
			require.Error(t, err)
			assert.NotErrorIs(t, err, storage.ErrNotBootstrapped, "corrupted state should not return ErrNotBootstrapped")
		})
	})

	t.Run("already bootstrapped loads correctly", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 5, nil, func(db storage.DB, _ storage.LockManager, _ *ContractDeploymentsIndex) {
			// Open the index again from the same DB — should succeed.
			idx2, err := NewContractDeploymentsIndex(db)
			require.NoError(t, err)
			assert.Equal(t, uint64(5), idx2.FirstIndexedHeight())
			assert.Equal(t, uint64(5), idx2.LatestIndexedHeight())
		})
	})
}

// ----------------------------------------------------------------------------
// BootstrapContractDeployments
// ----------------------------------------------------------------------------

func TestContractDeployments_Bootstrap(t *testing.T) {
	t.Parallel()

	t.Run("bootstrap initializes height markers", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 10, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			assert.Equal(t, uint64(10), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(10), idx.LatestIndexedHeight())
		})
	})

	t.Run("bootstrap at height 0", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 0, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			assert.Equal(t, uint64(0), idx.FirstIndexedHeight())
			assert.Equal(t, uint64(0), idx.LatestIndexedHeight())
		})
	})

	t.Run("bootstrap with initial deployments stores them", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(unittest.RandomAddressFixture(), "MyContract", 5, 0, 0)
		RunWithBootstrappedContractDeploymentsIndex(t, 5, []access.ContractDeployment{d}, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			result, err := idx.ByContract(d.Address, d.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d, result)
		})
	})

	t.Run("empty contract name returns error during bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			deployment := access.ContractDeployment{
				Address:      unittest.RandomAddressFixture(),
				ContractName: "",
				BlockHeight:  1,
			}

			err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					_, err := BootstrapContractDeployments(lctx, rw, storageDB, 1, []access.ContractDeployment{deployment})
					return err
				})
			})
			require.Error(t, err)
		})
	})

	t.Run("contract name containing dot returns error during bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			deployment := access.ContractDeployment{
				Address:      unittest.RandomAddressFixture(),
				ContractName: "My.Contract",
				BlockHeight:  1,
			}

			err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					_, err := BootstrapContractDeployments(lctx, rw, storageDB, 1, []access.ContractDeployment{deployment})
					return err
				})
			})
			require.Error(t, err)
		})
	})

	t.Run("double-bootstrap returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, _ *ContractDeploymentsIndex) {
			err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					_, bootstrapErr := BootstrapContractDeployments(lctx, rw, db, 1, nil)
					return bootstrapErr
				})
			})
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})
}

// ----------------------------------------------------------------------------
// ByContract
// ----------------------------------------------------------------------------

func TestContractDeployments_ByContract(t *testing.T) {
	t.Parallel()

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			_, err := idx.ByContract(unittest.RandomAddressFixture(), "NoSuchContract")
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})

	t.Run("returns most recent deployment when multiple exist", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		d1 := makeDeployment(addr, "MyContract", 2, 0, 0)
		d2 := makeDeployment(addr, "MyContract", 3, 0, 0)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))

			result, err := idx.ByContract(d2.Address, d2.ContractName)
			require.NoError(t, err)
			// Most recent is height 3
			assert.Equal(t, uint64(3), result.BlockHeight)
		})
	})

	t.Run("returns single deployment correctly", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(unittest.RandomAddressFixture(), "MyContract", 2, 1, 2)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			result, err := idx.ByContract(d.Address, d.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d, result)
		})
	})
}

// ----------------------------------------------------------------------------
// DeploymentsByContractID
// ----------------------------------------------------------------------------

func TestContractDeployments_DeploymentsByContractID(t *testing.T) {
	t.Parallel()

	t.Run("no deployments for contract returns empty results", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			collected, nextCursor := collectContractDeployments(t, idx, unittest.RandomAddressFixture(), "NoSuchContract", 10, nil, nil)
			assert.Empty(t, collected)
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("first page returns deployments in descending order", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		d1 := makeDeployment(addr, "MyContract", 2, 1, 4)
		d2 := makeDeployment(addr, "MyContract", 3, 2, 5)
		d3 := makeDeployment(addr, "MyContract", 4, 3, 6)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 4, []access.ContractDeployment{d3}))

			collected, nextCursor := collectContractDeployments(t, idx, addr, "MyContract", 10, nil, nil)
			require.Len(t, collected, 3)
			// Descending order: height 4, 3, 2
			assertDeployment(t, d3, collected[0])
			assertDeployment(t, d2, collected[1])
			assertDeployment(t, d1, collected[2])
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("has-more sets NextCursor pointing to first item of next page", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		name := "MyContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			// Store 3 deployments at heights 2, 3, 4
			for h := uint64(2); h <= 4; h++ {
				d := makeDeployment(addr, name, h, 0, 0)
				require.NoError(t, storeContractDeployments(t, lm, idx, h, []access.ContractDeployment{d}))
			}

			// Request page of 2 when 3 exist: returns [h=4, h=3], cursor points to h=2 (next page).
			collected, nextCursor := collectContractDeployments(t, idx, addr, name, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, uint64(2), nextCursor.BlockHeight)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		name := "MyContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			// Store 4 deployments at heights 2-5
			for h := uint64(2); h <= 5; h++ {
				d := makeDeployment(addr, name, h, 0, 0)
				require.NoError(t, storeContractDeployments(t, lm, idx, h, []access.ContractDeployment{d}))
			}

			// First page: limit=2, no cursor → [h=5, h=4], cursor → h=3
			collected1, nextCursor := collectContractDeployments(t, idx, addr, name, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			require.Equal(t, uint64(3), nextCursor.BlockHeight)
			require.Equal(t, uint32(0), nextCursor.TransactionIndex)
			require.Equal(t, uint32(0), nextCursor.EventIndex)

			// Second page: resume from cursor → [h=3, h=2]
			collected2, _ := collectContractDeployments(t, idx, addr, name, 2, nextCursor, nil)
			require.Len(t, collected2, 2)

			// Heights across both pages must be distinct and descending
			allHeights := []uint64{
				collected1[0].BlockHeight,
				collected1[1].BlockHeight,
				collected2[0].BlockHeight,
				collected2[1].BlockHeight,
			}
			assert.Equal(t, []uint64{5, 4, 3, 2}, allHeights)
		})
	})
}

// ----------------------------------------------------------------------------
// All
// ----------------------------------------------------------------------------

func TestContractDeployments_All(t *testing.T) {
	t.Parallel()

	t.Run("empty index returns empty results", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			collected, nextCursor := collectAllContracts(t, idx, 10, nil, nil)
			assert.Empty(t, collected)
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("returns latest per contract in ascending address/contract name order", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA1 := makeDeployment(addr, "AContract", 2, 0, 0)
			dA2 := makeDeployment(addr, "AContract", 3, 0, 0) // later update
			dB := makeDeployment(addr, "BContract", 2, 1, 0)
			dC := makeDeployment(addr, "CContract", 2, 2, 0)

			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA1, dB, dC}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{dA2}))

			collected, _ := collectAllContracts(t, idx, 10, nil, nil)
			require.Len(t, collected, 3)

			// Ascending contractID order; contractA shows the most recent deployment (dA2)
			assertDeployment(t, dA2, collected[0])
			assertDeployment(t, dB, collected[1])
			assertDeployment(t, dC, collected[2])
		})
	})

	t.Run("has-more sets NextCursor pointing to first item of next page", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr, "AContract", 2, 0, 0)
			dB := makeDeployment(addr, "BContract", 2, 1, 0)
			dC := makeDeployment(addr, "CContract", 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// limit=2, 3 exist: returns [A, B], cursor → C (first of next page)
			collected, nextCursor := collectAllContracts(t, idx, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, dC.Address, nextCursor.Address)
			assert.Equal(t, dC.ContractName, nextCursor.ContractName)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr, "AContract", 2, 0, 0)
			dB := makeDeployment(addr, "BContract", 2, 1, 0)
			dC := makeDeployment(addr, "CContract", 2, 2, 0)
			dD := makeDeployment(addr, "DContract", 2, 3, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC, dD}))

			// First page: [A, B], cursor → C
			collected1, nextCursor := collectAllContracts(t, idx, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			// Second page: resume from cursor → [C, D]
			collected2, _ := collectAllContracts(t, idx, 2, nextCursor, nil)
			require.Len(t, collected2, 2)

			names := []string{
				collected1[0].ContractName,
				collected1[1].ContractName,
				collected2[0].ContractName,
				collected2[1].ContractName,
			}
			assert.Equal(t, []string{"AContract", "BContract", "CContract", "DContract"}, names)
		})
	})

	t.Run("filter applied to results", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr, "AContract", 2, 0, 0)
			dB := makeDeployment(addr, "BContract", 2, 1, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB}))

			// Filter that only accepts AContract
			filter := func(d *access.ContractDeployment) bool {
				return d.Address == dA.Address && d.ContractName == dA.ContractName
			}

			collected, _ := collectAllContracts(t, idx, 10, nil, filter)
			require.Len(t, collected, 1)
			assertDeployment(t, dA, collected[0])
		})
	})
}

// ----------------------------------------------------------------------------
// ByAddress
// ----------------------------------------------------------------------------

func TestContractDeployments_ByAddress(t *testing.T) {
	t.Parallel()

	t.Run("returns only contracts for that address", func(t *testing.T) {
		t.Parallel()
		addr1 := unittest.RandomAddressFixture()
		addr2 := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr1, "ContractA", 2, 0, 0)
			dB := makeDeployment(addr1, "ContractB", 2, 1, 0)
			dC := makeDeployment(addr2, "ContractC", 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// Query addr1 - should get ContractA and ContractB only
			collected1, _ := collectContractsByAddress(t, idx, addr1, 10, nil, nil)
			require.Len(t, collected1, 2)
			for _, d := range collected1 {
				assert.Equal(t, addr1, d.Address, "deployment %s should belong to addr1", d.ContractName)
			}

			// Query addr2 - should get ContractC only
			collected2, _ := collectContractsByAddress(t, idx, addr2, 10, nil, nil)
			require.Len(t, collected2, 1)
			assertDeployment(t, dC, collected2[0])
		})
	})

	t.Run("returns empty when address has no contracts", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			addr := unittest.RandomAddressFixture()
			collected, nextCursor := collectContractsByAddress(t, idx, addr, 10, nil, nil)
			assert.Empty(t, collected)
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("has-more sets NextCursor pointing to first item of next page", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr, "ContractA", 2, 0, 0)
			dB := makeDeployment(addr, "ContractB", 2, 1, 0)
			dC := makeDeployment(addr, "ContractC", 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// limit=2, 3 exist: returns [A, B], cursor → C (first of next page)
			collected, nextCursor := collectContractsByAddress(t, idx, addr, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, dC.Address, nextCursor.Address)
			assert.Equal(t, dC.ContractName, nextCursor.ContractName)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(addr, "ContractA", 2, 0, 0)
			dB := makeDeployment(addr, "ContractB", 2, 1, 0)
			dC := makeDeployment(addr, "ContractC", 2, 2, 0)
			dD := makeDeployment(addr, "ContractD", 2, 3, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC, dD}))

			// First page: [A, B], cursor → C
			collected1, nextCursor := collectContractsByAddress(t, idx, addr, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			// Second page: resume from cursor → [C, D]
			collected2, _ := collectContractsByAddress(t, idx, addr, 2, nextCursor, nil)
			require.Len(t, collected2, 2)

			names := []string{
				collected1[0].ContractName,
				collected1[1].ContractName,
				collected2[0].ContractName,
				collected2[1].ContractName,
			}
			assert.Equal(t, []string{"ContractA", "ContractB", "ContractC", "ContractD"}, names)
		})
	})
}

// ----------------------------------------------------------------------------
// Store
// ----------------------------------------------------------------------------

func TestContractDeployments_Store(t *testing.T) {
	t.Parallel()

	t.Run("stores consecutive heights successfully", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			d2 := makeDeployment(addr, "MyContract", 2, 0, 0)
			d3 := makeDeployment(addr, "MyContract", 3, 0, 0)

			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d2}))
			assert.Equal(t, uint64(2), idx.LatestIndexedHeight())

			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d3}))
			assert.Equal(t, uint64(3), idx.LatestIndexedHeight())
		})
	})

	t.Run("duplicate height returns ErrAlreadyExists", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, nil))

			err := storeContractDeployments(t, lm, idx, 2, nil)
			require.ErrorIs(t, err, storage.ErrAlreadyExists)
		})
	})

	t.Run("non-consecutive height returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			err := storeContractDeployments(t, lm, idx, 5, nil)
			require.Error(t, err)
			assert.NotErrorIs(t, err, storage.ErrAlreadyExists,
				"non-consecutive height should not return ErrAlreadyExists")
		})
	})

	t.Run("empty contract name in batch returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			bad := access.ContractDeployment{
				Address:      unittest.RandomAddressFixture(),
				ContractName: "",
				BlockHeight:  2,
			}
			err := storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{bad})
			require.Error(t, err)
		})
	})

	t.Run("contract name containing dot in batch returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			bad := access.ContractDeployment{
				Address:      unittest.RandomAddressFixture(),
				ContractName: "My.Contract",
				BlockHeight:  2,
			}
			err := storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{bad})
			require.Error(t, err)
		})
	})

	t.Run("store without lock returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			lctx := lm.NewContext()
			defer lctx.Release()

			// lctx does not hold the required lock
			err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return idx.Store(lctx, rw, 2, nil)
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "missing required lock")
		})
	})

	t.Run("uncommitted batch does not advance latestHeight", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(db storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.Equal(t, uint64(1), idx.LatestIndexedHeight())

			batch := db.NewBatch()
			err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return idx.Store(lctx, batch, 2, nil)
			})
			require.NoError(t, err)

			// Close without committing
			require.NoError(t, batch.Close())

			assert.Equal(t, uint64(1), idx.LatestIndexedHeight(),
				"latestHeight must not advance when batch is not committed")
		})
	})
}

// ----------------------------------------------------------------------------
// IsDeleted
// ----------------------------------------------------------------------------

func TestContractDeployments_IsDeleted(t *testing.T) {
	t.Parallel()

	addr := unittest.RandomAddressFixture()
	name := "MyContract"

	t.Run("IsDeleted=true is persisted and returned by ByContract", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(addr, name, 2, 0, 0)
		d.IsDeleted = true

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			result, err := idx.ByContract(d.Address, d.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d, result)
		})
	})

	t.Run("IsDeleted=false is persisted and returned by ByContract", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(addr, name, 2, 0, 0)
		d.IsDeleted = false

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			result, err := idx.ByContract(d.Address, d.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d, result)
		})
	})

	t.Run("ByContract returns deleted deployment as most recent", func(t *testing.T) {
		t.Parallel()
		d1 := makeDeployment(addr, name, 2, 0, 0) // initial deploy
		d2 := makeDeployment(addr, name, 3, 0, 0) // deletion
		d2.IsDeleted = true

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))

			result, err := idx.ByContract(d2.Address, d2.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d2, result)
		})
	})

	t.Run("IsDeleted is preserved across deploy-delete-redeploy sequence", func(t *testing.T) {
		t.Parallel()
		d1 := makeDeployment(addr, name, 2, 0, 0) // initial deploy
		d2 := makeDeployment(addr, name, 3, 0, 0) // deletion
		d2.IsDeleted = true
		d3 := makeDeployment(addr, name, 4, 0, 0) // redeploy

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 4, []access.ContractDeployment{d3}))

			// Most recent is the redeploy at h=4, not deleted
			result, err := idx.ByContract(d3.Address, d3.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d3, result)

			// Full history shows the deletion at h=3
			collected, _ := collectContractDeployments(t, idx, addr, name, 10, nil, nil)
			require.Len(t, collected, 3)
			assertDeployment(t, d3, collected[0]) // h=4
			assertDeployment(t, d2, collected[1]) // h=3 (deleted)
			assertDeployment(t, d1, collected[2]) // h=2
		})
	})

	t.Run("IsDeleted=true persisted in bootstrap deployments", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(addr, name, 5, 0, 0)
		d.IsDeleted = true

		RunWithBootstrappedContractDeploymentsIndex(t, 5, []access.ContractDeployment{d}, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			result, err := idx.ByContract(d.Address, d.ContractName)
			require.NoError(t, err)
			assertDeployment(t, d, result)
		})
	})

	t.Run("All returns deleted deployments (filtering is at backend layer)", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(addr, name, 2, 0, 0)
		d.IsDeleted = true

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			collected, _ := collectAllContracts(t, idx, 10, nil, nil)
			require.Len(t, collected, 1)
			assertDeployment(t, d, collected[0])
		})
	})

	t.Run("ByAddress returns deleted deployments (filtering is at backend layer)", func(t *testing.T) {
		t.Parallel()
		d := makeDeployment(addr, name, 2, 0, 0)
		d.IsDeleted = true

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			collected, _ := collectContractsByAddress(t, idx, d.Address, 10, nil, nil)
			require.Len(t, collected, 1)
			assertDeployment(t, d, collected[0])
		})
	})
}

// ----------------------------------------------------------------------------
// Key codec
// ----------------------------------------------------------------------------

func TestContractDeployments_KeyCodec(t *testing.T) {
	t.Parallel()

	t.Run("roundtrip: makeContractDeploymentKey then decodeDeploymentCursor", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		name := "MyContract"
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeContractDeploymentKey(addr, name, height, txIndex, eventIndex)
		cursor, err := decodeDeploymentCursor(key)
		require.NoError(t, err)
		assert.Equal(t, addr, cursor.Address)
		assert.Equal(t, name, cursor.ContractName)
		assert.Equal(t, height, cursor.BlockHeight)
		assert.Equal(t, txIndex, cursor.TransactionIndex)
		assert.Equal(t, eventIndex, cursor.EventIndex)
	})

	t.Run("ones complement ensures descending height order", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		name := "MyContract"

		keyLow := makeContractDeploymentKey(addr, name, 100, 0, 0)
		keyHigh := makeContractDeploymentKey(addr, name, 200, 0, 0)

		// Higher height => smaller key (descending iteration)
		assert.True(t, string(keyHigh) < string(keyLow),
			"key for higher height should sort before key for lower height")
	})

	t.Run("malformed key too short returns error", func(t *testing.T) {
		t.Parallel()
		_, err := decodeDeploymentCursor(make([]byte, 5))
		require.Error(t, err)
	})

	t.Run("malformed key with wrong prefix byte returns error", func(t *testing.T) {
		t.Parallel()
		addr := unittest.RandomAddressFixture()
		key := makeContractDeploymentKey(addr, "MyContract", 1, 0, 0)
		key[0] = 0xFF
		_, err := decodeDeploymentCursor(key)
		require.Error(t, err)
	})
}
