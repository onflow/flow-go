package operation_test

import (
	"testing"
	"time"

<<<<<<< HEAD:storage/operation/headers_test.go
	"github.com/onflow/crypto"
=======
	"github.com/dgraph-io/badger/v2"
>>>>>>> feature/malleability:storage/badger/operation/headers_test.go
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

		_, lctx := unittest.LockManagerWithContext(t, storage.LockInsertBlock)
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
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
		collectionGuaranteeID := unittest.IdentifierFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexCollectionGuaranteeBlock(rw.Writer(), collectionID, headerID)
		})
		require.NoError(t, err)

		actualID := &flow.Identifier{}
		err = operation.LookupCollectionGuaranteeBlock(db.Reader(), collectionID, actualID)
		require.NoError(t, err)
		assert.Equal(t, headerID, *actualID)
	})
}

func TestBlockHeightIndexLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {

		height := uint64(1337)
		expected := flow.Identifier{0x01, 0x02, 0x03}

		_, lctx := unittest.LockManagerWithContext(t, storage.LockFinalizeBlock)
		defer lctx.Release()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.IndexFinalizedBlockByHeight(lctx, rw, height, expected)
		})
		require.NoError(t, err)

		var actual flow.Identifier
		err = operation.LookupBlockHeight(db.Reader(), height, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}
