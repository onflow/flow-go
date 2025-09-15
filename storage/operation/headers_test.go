package operation_test

import (
	"testing"
	"time"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHeaderInsertCheckRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := &flow.Header{
			HeaderBody: flow.HeaderBody{
				View:               1337,
				Timestamp:          uint64(time.Now().UnixMilli()),
				ParentID:           flow.Identifier{0x11},
				ParentVoterIndices: []byte{0x44},
				ParentVoterSigData: []byte{0x88},
				ProposerID:         flow.Identifier{0x33},
			},
			PayloadHash: flow.Identifier{0x22},
		}
		blockID := expected.ID()

		lockManager := storage.NewTestingLockManager()

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.InsertHeader(lctx, rw, expected.ID(), expected)
			})
		})
		require.NoError(t, err)

		var actual flow.Header
		err = operation.RetrieveHeader(db.Reader(), blockID, &actual)
		require.NoError(t, err)

		assert.Equal(t, *expected, actual)
	})
}

func TestHeaderIDIndexByCollectionID(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		headerID := unittest.IdentifierFixture()
		collectionGuaranteeID := unittest.IdentifierFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexBlockContainingCollectionGuarantee(rw.Writer(), collectionGuaranteeID, headerID)
		})
		require.NoError(t, err)

		actualID := &flow.Identifier{}
		err = operation.LookupBlockContainingCollectionGuarantee(db.Reader(), collectionGuaranteeID, actualID)
		require.NoError(t, err)
		assert.Equal(t, headerID, *actualID)
	})
}

func TestBlockHeightIndexLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		height := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		lockManager := storage.NewTestingLockManager()

		err := unittest.WithLock(t, lockManager, storage.LockFinalizeBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexFinalizedBlockByHeight(lctx, rw, height, expected)
			})
		})
		require.NoError(t, err)

		var actual flow.Identifier
		err = operation.LookupBlockHeight(db.Reader(), height, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}

func TestBlockViewIndexLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		view := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		lockManager := storage.NewTestingLockManager()

		err := unittest.WithLock(t, lockManager, storage.LockInsertBlock, func(lctx lockctx.Context) error {
			return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
				return operation.IndexCertifiedBlockByView(lctx, rw, view, expected)
			})
		})
		require.NoError(t, err)

		var actual flow.Identifier
		err = operation.LookupCertifiedBlockByView(db.Reader(), view, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}
