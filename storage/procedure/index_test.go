package procedure

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestInsertRetrieveIndex(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		blockID := unittest.IdentifierFixture()
		index := unittest.IndexFixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return InsertIndex(rw, blockID, index)
		})
		require.NoError(t, err)

		var retrieved flow.Index
		err = RetrieveIndex(db.Reader(), blockID, &retrieved)
		require.NoError(t, err)

		require.Equal(t, index, &retrieved)
	})
}
