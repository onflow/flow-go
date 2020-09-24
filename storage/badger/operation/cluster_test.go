package operation_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterHeights(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		var (
			clusterID flow.ChainID = "cluster"
			height    uint64       = 42
			expected               = unittest.IdentifierFixture()
			err       error
		)

		t.Run("retrieve non-existent", func(t *testing.T) {
			var actual flow.Identifier
			err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
			t.Log(err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			err = db.Update(operation.IndexClusterBlockHeight(clusterID, height, expected))
			assert.Nil(t, err)

			var actual flow.Identifier
			err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
			assert.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				// use different cluster ID but same block height
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = unittest.IdentifierFixture()

				var actual flow.Identifier
				err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
				assert.True(t, errors.Is(err, storage.ErrNotFound))

				err = db.Update(operation.IndexClusterBlockHeight(clusterID, height, expected))
				assert.Nil(t, err)

				err = db.View(operation.LookupClusterBlockHeight(clusterID, height, &actual))
				assert.Nil(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}

func TestClusterBoundaries(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		var (
			clusterID flow.ChainID = "cluster"
			expected  uint64       = 42
			err       error
		)

		t.Run("retrieve non-existant", func(t *testing.T) {
			var actual uint64
			err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
			t.Log(err)
			assert.True(t, errors.Is(err, storage.ErrNotFound))
		})

		t.Run("insert/retrieve", func(t *testing.T) {
			err = db.Update(operation.InsertClusterFinalizedHeight(clusterID, 21))
			assert.Nil(t, err)

			err = db.Update(operation.UpdateClusterFinalizedHeight(clusterID, expected))
			assert.Nil(t, err)

			var actual uint64
			err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
			assert.Nil(t, err)
			assert.Equal(t, expected, actual)
		})

		t.Run("multiple chain IDs", func(t *testing.T) {
			for i := 0; i < 3; i++ {
				// use different cluster ID but same boundary
				clusterID = flow.ChainID(fmt.Sprintf("cluster-%d", i))
				expected = uint64(i)

				var actual uint64
				err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
				assert.True(t, errors.Is(err, storage.ErrNotFound))

				err = db.Update(operation.InsertClusterFinalizedHeight(clusterID, expected))
				assert.Nil(t, err)

				err = db.View(operation.RetrieveClusterFinalizedHeight(clusterID, &actual))
				assert.Nil(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	})
}
