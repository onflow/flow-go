package badger

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBootstrap(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		state, err := NewState(db)
		require.Nil(t, err)
		mutator := Mutator{
			state: state,
		}

		t.Run("invalid number", func(t *testing.T) {
			genesis := cluster.Genesis()
			genesis.Number = 1

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("invalid parent hash", func(t *testing.T) {
			genesis := cluster.Genesis()
			genesis.ParentID = unittest.IdentifierFixture()

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("payload hash does not match payload", func(t *testing.T) {
			genesis := cluster.Genesis()
			genesis.PayloadHash = unittest.IdentifierFixture()

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("invalid payload", func(t *testing.T) {
			genesis := cluster.Genesis()
			genesis.Payload = cluster.Payload{
				Collection: &flow.LightCollection{
					Transactions: []flow.Identifier{unittest.IdentifierFixture()},
				},
			}

			err := mutator.Bootstrap(genesis)
			assert.Error(t, err)
		})

		t.Run("bootstrap", func(t *testing.T) {
			genesis := cluster.Genesis()

			err := mutator.Bootstrap(genesis)
			assert.Nil(t, err)

			err = db.View(func(tx *badger.Txn) error {

				// should insert collection
				var collection flow.LightCollection
				err = operation.RetrieveCollection(genesis.Collection.ID(), &collection)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Collection, &collection)

				// should index collection
				var collectionID flow.Identifier
				err = operation.LookupCollection(genesis.PayloadHash, &collectionID)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Collection.ID(), collectionID)

				// should insert header
				var header flow.Header
				err = operation.RetrieveHeader(genesis.ID(), &header)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Header.ID(), header.ID())

				// should insert block number -> ID lookup
				var blockID flow.Identifier
				err = operation.RetrieveNumberForCluster(genesis.ChainID, genesis.Number, &blockID)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.ID(), blockID)

				// should insert boundary
				var boundary uint64
				err = operation.RetrieveBoundaryForCluster(genesis.ChainID, &boundary)(tx)
				assert.Nil(t, err)
				assert.Equal(t, genesis.Number, boundary)

				return nil
			})
			assert.Nil(t, err)
		})
	})
}

func TestExtend(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		t.Run("without first bootstrapping", func(t *testing.T) {})

		// bootstrap cluster state

		t.Run("non-existent block", func(t *testing.T) {})

		t.Run("invalid payload hash", func(t *testing.T) {})

		t.Run("non-existent parent", func(t *testing.T) {})

		t.Run("invalid block number", func(t *testing.T) {})

		t.Run("building on parent of finalized block", func(t *testing.T) {})

		t.Run("extend", func(t *testing.T) {})
	})
}
