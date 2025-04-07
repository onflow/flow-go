package operation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
	"github.com/onflow/flow-go/storage/operation/dbtest"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSealInsertCheckRetrieve(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		expected := unittest.Seal.Fixture()

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return operation.InsertSeal(rw.Writer(), expected)
		})
		require.NoError(t, err)

		var actual flow.Seal
		err = operation.RetrieveSeal(db.Reader(), expected.ID(), &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, &actual)
	})
}

func TestSealIndexAndLookup(t *testing.T) {
	dbtest.RunWithDB(t, func(t *testing.T, db storage.DB) {
		seal1 := unittest.Seal.Fixture()
		seal2 := unittest.Seal.Fixture()

		seals := []*flow.Seal{seal1, seal2}

		blockID := flow.MakeID([]byte{0x42})

		expected := []flow.Identifier(flow.GetIDs(seals))

		err := db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			for _, seal := range seals {
				if err := operation.InsertSeal(rw.Writer(), seal); err != nil {
					return err
				}
			}
			if err := operation.IndexPayloadSeals(rw.Writer(), blockID, expected); err != nil {
				return err
			}
			return nil
		})
		require.NoError(t, err)

		var actual []flow.Identifier
		err = operation.LookupPayloadSeals(db.Reader(), blockID, &actual)
		require.NoError(t, err)

		assert.Equal(t, expected, actual)
	})
}
