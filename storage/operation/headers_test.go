package operation_test

import (
	"testing"
	"time"

	"github.com/onflow/crypto"
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
			View:               1337,
			Timestamp:          time.Now().UTC(),
			ParentID:           flow.Identifier{0x11},
			PayloadHash:        flow.Identifier{0x22},
			ParentVoterIndices: []byte{0x44},
			ParentVoterSigData: []byte{0x88},
			ProposerID:         flow.Identifier{0x33},
			ProposerSigData:    crypto.Signature{0x77},
		}
		blockID := expected.ID()

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockInsertBlock)
		require.NoError(t, err)
		defer lctx.Release()

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertHeader(lctx, rw, expected.ID(), expected)
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
		collectionID := unittest.IdentifierFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexCollectionBlock(rw.Writer(), collectionID, headerID)
		})
		require.NoError(t, err)

		actualID := &flow.Identifier{}
		err = operation.LookupBlockContainingCollection(db.Reader(), collectionID, actualID)
		require.NoError(t, err)
		assert.Equal(t, headerID, *actualID)
	})
}

func TestBlockHeightIndexLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		height := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		lockManager := storage.NewTestingLockManager()
		lctx := lockManager.NewContext()
		err := lctx.AcquireLock(storage.LockFinalizeBlock)
		require.NoError(t, err)
		defer lctx.Release()

		err = db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx, rw, height, expected)
		})
		require.NoError(t, err)

		var actual flow.Identifier
		err = operation.LookupBlockHeight(db.Reader(), height, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}
