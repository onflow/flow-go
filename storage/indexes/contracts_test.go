package indexes

import (
	"strings"
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

// collectContractDeployments is a test helper that creates a DeploymentsByContractID iterator
// and collects results via CollectResults.
func collectContractDeployments(
	tb testing.TB,
	idx *ContractDeploymentsIndex,
	id string,
	limit uint32,
	cursor *access.ContractDeploymentsCursor,
	filter storage.IndexFilter[*access.ContractDeployment],
) ([]access.ContractDeployment, *access.ContractDeploymentsCursor) {
	tb.Helper()
	iter, err := idx.DeploymentsByContractID(id, cursor)
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

// makeDeployment builds a minimal access.ContractDeployment for use in tests.
func makeDeployment(contractID string, height uint64, txIndex, eventIndex uint32) access.ContractDeployment {
	parts := strings.Split(contractID, ".")
	var addr flow.Address
	if len(parts) >= 2 {
		parsed, err := flow.StringToAddress(parts[1])
		if err == nil {
			addr = parsed
		}
	}
	fakeHash := unittest.IdentifierFixture()
	return access.ContractDeployment{
		ContractID:       contractID,
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
			assert.False(t, isNotBootstrapped(err),
				"corrupted state should not return ErrNotBootstrapped")
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

// isNotBootstrapped is a helper to avoid importing errors in the test body.
func isNotBootstrapped(err error) bool {
	return err != nil && strings.Contains(err.Error(), storage.ErrNotBootstrapped.Error())
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
		d := makeDeployment("A.1234567890abcdef.MyContract", 5, 0, 0)
		RunWithBootstrappedContractDeploymentsIndex(t, 5, []access.ContractDeployment{d}, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			result, err := idx.ByContractID(d.ContractID)
			require.NoError(t, err)
			assert.Equal(t, d.ContractID, result.ContractID)
			assert.Equal(t, d.BlockHeight, result.BlockHeight)
			assert.Equal(t, d.TransactionIndex, result.TransactionIndex)
			assert.Equal(t, d.EventIndex, result.EventIndex)
		})
	})

	t.Run("short contractID returns error during bootstrap", func(t *testing.T) {
		t.Parallel()
		unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
			storageDB := pebbleimpl.ToDB(db)
			lm := storage.NewTestingLockManager()

			shortID := access.ContractDeployment{
				ContractID:  "A.short",
				BlockHeight: 1,
			}

			err := unittest.WithLock(t, lm, storage.LockIndexContractDeployments, func(lctx lockctx.Context) error {
				return storageDB.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
					_, err := BootstrapContractDeployments(lctx, rw, storageDB, 1, []access.ContractDeployment{shortID})
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
// ByContractID
// ----------------------------------------------------------------------------

func TestContractDeployments_ByContractID(t *testing.T) {
	t.Parallel()

	t.Run("not found returns ErrNotFound", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, _ storage.LockManager, idx *ContractDeploymentsIndex) {
			_, err := idx.ByContractID("A.1234567890abcdef.NoSuchContract")
			require.ErrorIs(t, err, storage.ErrNotFound)
		})
	})

	t.Run("returns most recent deployment when multiple exist", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"
		d1 := makeDeployment(contractID, 2, 0, 0)
		d2 := makeDeployment(contractID, 3, 0, 0)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))

			result, err := idx.ByContractID(contractID)
			require.NoError(t, err)
			// Most recent is height 3
			assert.Equal(t, uint64(3), result.BlockHeight)
		})
	})

	t.Run("returns single deployment correctly", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"
		d := makeDeployment(contractID, 2, 1, 2)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d}))

			result, err := idx.ByContractID(contractID)
			require.NoError(t, err)
			assert.Equal(t, contractID, result.ContractID)
			assert.Equal(t, uint64(2), result.BlockHeight)
			assert.Equal(t, uint32(1), result.TransactionIndex)
			assert.Equal(t, uint32(2), result.EventIndex)
			assert.Equal(t, d.TransactionID, result.TransactionID)
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
			collected, nextCursor := collectContractDeployments(t, idx, "A.1234567890abcdef.NoSuchContract", 10, nil, nil)
			assert.Empty(t, collected)
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("first page returns deployments in descending order", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"
		d1 := makeDeployment(contractID, 2, 0, 0)
		d2 := makeDeployment(contractID, 3, 0, 0)
		d3 := makeDeployment(contractID, 4, 0, 0)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{d1}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{d2}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 4, []access.ContractDeployment{d3}))

			collected, nextCursor := collectContractDeployments(t, idx, contractID, 10, nil, nil)
			require.Len(t, collected, 3)
			// Descending order: height 4, 3, 2
			assert.Equal(t, uint64(4), collected[0].BlockHeight)
			assert.Equal(t, uint64(3), collected[1].BlockHeight)
			assert.Equal(t, uint64(2), collected[2].BlockHeight)
			assert.Nil(t, nextCursor)
		})
	})

	t.Run("has-more sets NextCursor pointing to first item of next page", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			// Store 3 deployments at heights 2, 3, 4
			for h := uint64(2); h <= 4; h++ {
				d := makeDeployment(contractID, h, 0, 0)
				require.NoError(t, storeContractDeployments(t, lm, idx, h, []access.ContractDeployment{d}))
			}

			// Request page of 2 when 3 exist: returns [h=4, h=3], cursor points to h=2 (next page).
			collected, nextCursor := collectContractDeployments(t, idx, contractID, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, uint64(2), nextCursor.BlockHeight)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			// Store 4 deployments at heights 2-5
			for h := uint64(2); h <= 5; h++ {
				d := makeDeployment(contractID, h, 0, 0)
				require.NoError(t, storeContractDeployments(t, lm, idx, h, []access.ContractDeployment{d}))
			}

			// First page: limit=2, no cursor → [h=5, h=4], cursor → h=3
			collected1, nextCursor := collectContractDeployments(t, idx, contractID, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			require.Equal(t, uint64(3), nextCursor.BlockHeight)
			require.Equal(t, uint32(0), nextCursor.TransactionIndex)
			require.Equal(t, uint32(0), nextCursor.EventIndex)

			// Second page: resume from cursor → [h=3, h=2]
			collected2, _ := collectContractDeployments(t, idx, contractID, 2, nextCursor, nil)
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

	t.Run("returns latest per contract in ascending contract ID order", func(t *testing.T) {
		t.Parallel()
		// Use contractIDs with different names so lexicographic order is predictable
		contractA := "A.1234567890abcdef.AContract"
		contractB := "A.1234567890abcdef.BContract"
		contractC := "A.1234567890abcdef.CContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA1 := makeDeployment(contractA, 2, 0, 0)
			dA2 := makeDeployment(contractA, 3, 0, 0) // later update
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)

			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA1, dB, dC}))
			require.NoError(t, storeContractDeployments(t, lm, idx, 3, []access.ContractDeployment{dA2}))

			collected, _ := collectAllContracts(t, idx, 10, nil, nil)
			require.Len(t, collected, 3)

			// Ascending contractID order
			assert.Equal(t, contractA, collected[0].ContractID)
			assert.Equal(t, contractB, collected[1].ContractID)
			assert.Equal(t, contractC, collected[2].ContractID)

			// contractA shows the most recent deployment (height 3)
			assert.Equal(t, uint64(3), collected[0].BlockHeight)
		})
	})

	t.Run("has-more sets NextCursor pointing to first item of next page", func(t *testing.T) {
		t.Parallel()
		contractA := "A.1234567890abcdef.AContract"
		contractB := "A.1234567890abcdef.BContract"
		contractC := "A.1234567890abcdef.CContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// limit=2, 3 exist: returns [A, B], cursor → C (first of next page)
			collected, nextCursor := collectAllContracts(t, idx, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, contractC, nextCursor.ContractID)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		contractA := "A.1234567890abcdef.AContract"
		contractB := "A.1234567890abcdef.BContract"
		contractC := "A.1234567890abcdef.CContract"
		contractD := "A.1234567890abcdef.DContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)
			dD := makeDeployment(contractD, 2, 3, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC, dD}))

			// First page: [A, B], cursor → C
			collected1, nextCursor := collectAllContracts(t, idx, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			// Second page: resume from cursor → [C, D]
			collected2, _ := collectAllContracts(t, idx, 2, nextCursor, nil)
			require.Len(t, collected2, 2)

			ids := []string{
				collected1[0].ContractID,
				collected1[1].ContractID,
				collected2[0].ContractID,
				collected2[1].ContractID,
			}
			assert.Equal(t, []string{contractA, contractB, contractC, contractD}, ids)
		})
	})

	t.Run("filter applied to results", func(t *testing.T) {
		t.Parallel()
		contractA := "A.1234567890abcdef.AContract"
		contractB := "A.1234567890abcdef.BContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB}))

			// Filter that only accepts contractA
			filter := func(d *access.ContractDeployment) bool {
				return d.ContractID == contractA
			}

			collected, _ := collectAllContracts(t, idx, 10, nil, filter)
			require.Len(t, collected, 1)
			assert.Equal(t, contractA, collected[0].ContractID)
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
		// Two different address hex values
		addrHex1 := "1234567890abcdef"
		addrHex2 := "fedcba0987654321"

		contractA := "A." + addrHex1 + ".ContractA"
		contractB := "A." + addrHex1 + ".ContractB"
		contractC := "A." + addrHex2 + ".ContractC"

		addr1, err := flow.StringToAddress(addrHex1)
		require.NoError(t, err)
		addr2, err := flow.StringToAddress(addrHex2)
		require.NoError(t, err)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// Query addr1 - should get ContractA and ContractB only
			collected1, _ := collectContractsByAddress(t, idx, addr1, 10, nil, nil)
			require.Len(t, collected1, 2)
			for _, d := range collected1 {
				assert.True(t, strings.Contains(d.ContractID, addrHex1),
					"deployment %s should belong to addr1", d.ContractID)
			}

			// Query addr2 - should get ContractC only
			collected2, _ := collectContractsByAddress(t, idx, addr2, 10, nil, nil)
			require.Len(t, collected2, 1)
			assert.Equal(t, contractC, collected2[0].ContractID)
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
		addrHex := "1234567890abcdef"
		contractA := "A." + addrHex + ".ContractA"
		contractB := "A." + addrHex + ".ContractB"
		contractC := "A." + addrHex + ".ContractC"

		addr, err := flow.StringToAddress(addrHex)
		require.NoError(t, err)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC}))

			// limit=2, 3 exist: returns [A, B], cursor → C (first of next page)
			collected, nextCursor := collectContractsByAddress(t, idx, addr, 2, nil, nil)
			require.Len(t, collected, 2)
			require.NotNil(t, nextCursor)
			assert.Equal(t, contractC, nextCursor.ContractID)
		})
	})

	t.Run("with cursor resumes from cursor position (inclusive)", func(t *testing.T) {
		t.Parallel()
		addrHex := "1234567890abcdef"
		contractA := "A." + addrHex + ".ContractA"
		contractB := "A." + addrHex + ".ContractB"
		contractC := "A." + addrHex + ".ContractC"
		contractD := "A." + addrHex + ".ContractD"

		addr, err := flow.StringToAddress(addrHex)
		require.NoError(t, err)

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			dA := makeDeployment(contractA, 2, 0, 0)
			dB := makeDeployment(contractB, 2, 1, 0)
			dC := makeDeployment(contractC, 2, 2, 0)
			dD := makeDeployment(contractD, 2, 3, 0)
			require.NoError(t, storeContractDeployments(t, lm, idx, 2, []access.ContractDeployment{dA, dB, dC, dD}))

			// First page: [A, B], cursor → C
			collected1, nextCursor := collectContractsByAddress(t, idx, addr, 2, nil, nil)
			require.Len(t, collected1, 2)
			require.NotNil(t, nextCursor)

			// Second page: resume from cursor → [C, D]
			collected2, _ := collectContractsByAddress(t, idx, addr, 2, nextCursor, nil)
			require.Len(t, collected2, 2)

			ids := []string{
				collected1[0].ContractID,
				collected1[1].ContractID,
				collected2[0].ContractID,
				collected2[1].ContractID,
			}
			assert.Equal(t, []string{contractA, contractB, contractC, contractD}, ids)
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
		contractID := "A.1234567890abcdef.MyContract"

		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			d2 := makeDeployment(contractID, 2, 0, 0)
			d3 := makeDeployment(contractID, 3, 0, 0)

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

	t.Run("short contractID in batch returns error", func(t *testing.T) {
		t.Parallel()
		RunWithBootstrappedContractDeploymentsIndex(t, 1, nil, func(_ storage.DB, lm storage.LockManager, idx *ContractDeploymentsIndex) {
			bad := access.ContractDeployment{
				ContractID:  "A.short",
				BlockHeight: 2,
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
// Key codec
// ----------------------------------------------------------------------------

func TestContractDeployments_KeyCodec(t *testing.T) {
	t.Parallel()

	t.Run("roundtrip: makeContractDeploymentKey then decodeContractDeploymentKey", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"
		height := uint64(12345)
		txIndex := uint32(42)
		eventIndex := uint32(7)

		key := makeContractDeploymentKey(contractID, height, txIndex, eventIndex)
		gotContractID, gotHeight, gotTxIndex, gotEventIndex, err := decodeContractDeploymentKey(key)
		require.NoError(t, err)
		assert.Equal(t, contractID, gotContractID)
		assert.Equal(t, height, gotHeight)
		assert.Equal(t, txIndex, gotTxIndex)
		assert.Equal(t, eventIndex, gotEventIndex)
	})

	t.Run("ones complement ensures descending height order", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"

		keyLow := makeContractDeploymentKey(contractID, 100, 0, 0)
		keyHigh := makeContractDeploymentKey(contractID, 200, 0, 0)

		// Higher height => smaller key (descending iteration)
		assert.True(t, string(keyHigh) < string(keyLow),
			"key for higher height should sort before key for lower height")
	})

	t.Run("malformed key too short returns error", func(t *testing.T) {
		t.Parallel()
		_, _, _, _, err := decodeContractDeploymentKey(make([]byte, 5))
		require.Error(t, err)
	})

	t.Run("malformed key with wrong prefix byte returns error", func(t *testing.T) {
		t.Parallel()
		contractID := "A.1234567890abcdef.MyContract"
		key := makeContractDeploymentKey(contractID, 1, 0, 0)
		key[0] = 0xFF
		_, _, _, _, err := decodeContractDeploymentKey(key)
		require.Error(t, err)
	})

	t.Run("addressFromContractID parses valid ID", func(t *testing.T) {
		t.Parallel()
		addrHex := "1234567890abcdef"
		contractID := "A." + addrHex + ".MyContract"
		expected, err := flow.StringToAddress(addrHex)
		require.NoError(t, err)

		got, err := addressFromContractID(contractID)
		require.NoError(t, err)
		assert.Equal(t, expected, got)
	})

	t.Run("addressFromContractID rejects missing A. prefix", func(t *testing.T) {
		t.Parallel()
		_, err := addressFromContractID("1234567890abcdef.MyContract")
		require.Error(t, err)
	})

	t.Run("addressFromContractID rejects missing second dot", func(t *testing.T) {
		t.Parallel()
		_, err := addressFromContractID("A.1234567890abcdef")
		require.Error(t, err)
	})

	t.Run("addressFromContractID rejects invalid hex address", func(t *testing.T) {
		t.Parallel()
		_, err := addressFromContractID("A.ZZZZZZZZZZZZZZZZ.MyContract")
		require.Error(t, err)
	})
}

