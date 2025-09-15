package consensus

import (
	"math/rand"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	mockprot "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/storage/store"
	"github.com/onflow/flow-go/utils/unittest"
)

func LogCleanup(list *[]flow.Identifier) func(flow.Identifier) error {
	return func(blockID flow.Identifier) error {
		*list = append(*list, blockID)
		return nil
	}
}

// TestMakeFinalValidChain checks whether calling `MakeFinal` with the ID of a valid
// descendant block of the latest finalized header results in the finalization of the
// valid descendant and all of its parents up to the finalized header, but excluding
// the children of the valid descendant.
func TestMakeFinalValidChain(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// generate a couple of children that are pending
	parent := final
	var pending []*flow.Header
	total := 8
	for i := 0; i < total; i++ {
		header := unittest.BlockHeaderFixture()
		header.Height = parent.Height + 1
		header.ParentID = parent.ID()
		pending = append(pending, header)
		parent = header
	}

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// make sure we get a finalize call for the blocks that we want to
	cutoff := total - 3
	var lastID flow.Identifier
	for i := 0; i < cutoff; i++ {
		state.On("Finalize", mock.Anything, pending[i].ID()).Return(nil)
		lastID = pending[i].ID()
	}

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		// set up lock context
		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockFinalizeBlock)
		require.NoError(t, err)
		defer lctx.Release()

		dbImpl := pebbleimpl.ToDB(pdb)

		// insert the latest finalized height
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.UpsertFinalizedHeight(lctx, rw.Writer(), final.Height)
		})
		require.NoError(t, err)

		// map the finalized height to the finalized block ID
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx, rw, final.Height, final.ID())
		})
		require.NoError(t, err)

		// insert the finalized block header into the DB
		insertLctx := lockManager.NewContext()
		require.NoError(t, insertLctx.AcquireLock(storage.LockInsertBlock))
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(insertLctx, rw, final.ID(), final)
		})
		require.NoError(t, err)
		insertLctx.Release()

		// insert all of the pending blocks into the DB
		for _, header := range pending {
			insertLctx2 := lockManager.NewContext()
			require.NoError(t, insertLctx2.AcquireLock(storage.LockInsertBlock))
			err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertHeader(insertLctx2, rw, header.ID(), header)
			})
			require.NoError(t, err)
			insertLctx2.Release()
		}

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := Finalizer{
			dbReader: pebbleimpl.ToDB(pdb).Reader(),
			headers:  store.NewHeaders(metrics, pebbleimpl.ToDB(pdb)),
			state:    state,
			tracer:   trace.NewNoopTracer(),
			cleanup:  LogCleanup(&list),
		}
		err = fin.MakeFinal(lastID)
		require.NoError(t, err)
	})

	// make sure that finalize was called on protocol state for all desired blocks
	state.AssertExpectations(t)

	// make sure that cleanup was called for all of them too
	assert.ElementsMatch(t, list, flow.GetIDs(pending[:cutoff]))
}

// TestMakeFinalInvalidHeight checks whether we receive an error when calling `MakeFinal`
// with a header that is at the same height as the already highest finalized header.
func TestMakeFinalInvalidHeight(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// generate an alternative block at same height
	pending := unittest.BlockHeaderFixture()
	pending.Height = final.Height

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		dbImpl := pebbleimpl.ToDB(pdb)
		lockManager := storage.NewTestingLockManager()

		// Insert the latest finalized height and map the finalized height to the finalized block ID.
		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockFinalizeBlock))
		err := dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := operation.IndexFinalizedBlockByHeight(lctx, rw, final.Height, final.ID())
			if err != nil {
				return err
			}
			return operation.UpsertFinalizedHeight(lctx, rw.Writer(), final.Height)
		})
		require.NoError(t, err)
		// NOTE: must release lock here - do not deferr! Reason:
		// The business logic we are testing here should not be expected to do anything regarding finalization when
		// somebody else is still holding the lock `LockFinalizeBlock`. However, we want to verify that finalizing
		// another block at the same height is rejected. Hence, we should not be holding the lock anymore.
		lctx.Release()

		// Insert the latest finalized height and map the finalized height to the finalized block ID.
		insertLctx := lockManager.NewContext()
		require.NoError(t, insertLctx.AcquireLock(storage.LockInsertBlock))
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(insertLctx, rw, final.ID(), final)
		})
		require.NoError(t, err)
		insertLctx.Release()

		// insert all of the pending header into DB
		insertLctx = lockManager.NewContext()
		require.NoError(t, insertLctx.AcquireLock(storage.LockInsertBlock))
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(insertLctx, rw, pending.ID(), pending)
		})
		require.NoError(t, err)
		insertLctx.Release()

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := Finalizer{
			dbReader: pebbleimpl.ToDB(pdb).Reader(),
			headers:  store.NewHeaders(metrics, pebbleimpl.ToDB(pdb)),
			state:    state,
			tracer:   trace.NewNoopTracer(),
			cleanup:  LogCleanup(&list),
		}
		err = fin.MakeFinal(pending.ID())
		require.Error(t, err)
	})

	// make sure that nothing was finalized
	state.AssertExpectations(t)

	// make sure no cleanup was done
	assert.Empty(t, list)
}

// TestMakeFinalDuplicate checks whether calling `MakeFinal` with the ID of the currently
// highest finalized header is a no-op and does not result in an error.
func TestMakeFinalDuplicate(t *testing.T) {

	// create one block that we consider the last finalized
	final := unittest.BlockHeaderFixture()
	final.Height = uint64(rand.Uint32())

	// create a mock protocol state to check finalize calls
	state := mockprot.NewFollowerState(t)

	// this will hold the IDs of blocks clean up
	var list []flow.Identifier

	unittest.RunWithPebbleDB(t, func(pdb *pebble.DB) {
		lockManager := storage.NewTestingLockManager()
		dbImpl := pebbleimpl.ToDB(pdb)

		// insert the latest finalized height
		lctx := lockManager.NewContext()
		require.NoError(t, lctx.AcquireLock(storage.LockFinalizeBlock))
		err := dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			err := operation.UpsertFinalizedHeight(lctx, rw.Writer(), final.Height)
			if err != nil {
				return err
			}

			return operation.IndexFinalizedBlockByHeight(lctx, rw, final.Height, final.ID())
		})
		require.NoError(t, err)
		// NOTE: must release lock here - do not deferr! Reason:
		// The business logic we are testing here should not be expected to do anything regarding finalization when
		// somebody else is still holding the lock `LockFinalizeBlock`. However, we want to verify that finalizing
		// the same block again is a no-op. Hence, we should not be holding the lock anymore.
		lctx.Release()

		// insert the finalized block header into the DB
		insertLctx := lockManager.NewContext()
		require.NoError(t, insertLctx.AcquireLock(storage.LockInsertBlock))
		err = dbImpl.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(insertLctx, rw, final.ID(), final)
		})
		require.NoError(t, err)
		insertLctx.Release()

		// initialize the finalizer with the dependencies and make the call
		metrics := metrics.NewNoopCollector()
		fin := Finalizer{
			dbReader: pebbleimpl.ToDB(pdb).Reader(),
			headers:  store.NewHeaders(metrics, pebbleimpl.ToDB(pdb)),
			state:    state,
			tracer:   trace.NewNoopTracer(),
			cleanup:  LogCleanup(&list),
		}
		err = fin.MakeFinal(final.ID())
		require.NoError(t, err)
	})

	// make sure that nothing was finalized
	state.AssertExpectations(t)

	// make sure no cleanup was done
	assert.Empty(t, list)
}
