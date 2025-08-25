package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestLightCollectionID_Malleability confirms that the LightCollection struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestLightCollectionID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, &flow.LightCollection{
		Transactions: unittest.IdentifierListFixture(5),
	})
}

// TestCollectionID_Malleability confirms that the Collection struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestCollectionID_Malleability(t *testing.T) {
	collection := unittest.CollectionFixture(5)
	unittest.RequireEntityNonMalleable(t, &collection, unittest.WithTypeGenerator[flow.TransactionBody](func() flow.TransactionBody {
		return unittest.TransactionBodyFixture()
	}))
}

func TestNewCollection(t *testing.T) {
	t.Run("valid untrusted collection", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		ub := flow.UntrustedCollection{
			Transactions: []*flow.TransactionBody{&tx},
		}

		col, err := flow.NewCollection(ub)
		assert.NoError(t, err)
		assert.NotNil(t, col)
		assert.Len(t, col.Transactions, 1)

		t.Run("convert to LightCollection", func(t *testing.T) {
			light := col.Light()
			assert.Equal(t, light.Len(), col.Len())
			for i := range light.Len() {
				assert.Equal(t, col.Transactions[i].ID(), light.Transactions[i])
			}
		})
	})

	t.Run("empty transaction list", func(t *testing.T) {
		ub := flow.UntrustedCollection{
			Transactions: []*flow.TransactionBody{},
		}

		col, err := flow.NewCollection(ub)
		assert.NoError(t, err)
		assert.NotNil(t, col)
		assert.Empty(t, col.Transactions)
	})

	t.Run("nil transaction list", func(t *testing.T) {
		ub := flow.UntrustedCollection{
			Transactions: nil,
		}

		col, err := flow.NewCollection(ub)
		assert.NoError(t, err)
		assert.NotNil(t, col)
		assert.Empty(t, col.Transactions)
	})

	t.Run("nil transaction in list", func(t *testing.T) {
		ub := flow.UntrustedCollection{
			Transactions: []*flow.TransactionBody{nil},
		}

		col, err := flow.NewCollection(ub)
		assert.Error(t, err)
		assert.Nil(t, col)
		assert.Contains(t, err.Error(), "transaction at index")
	})
}

// TestNewLightCollection tests creating a new LightCollection.
// All possible inputs should produce a valid LightCollection, including nil/empty transaction lists.
func TestNewLightCollection(t *testing.T) {
	t.Run("valid untrusted light collection", func(t *testing.T) {
		untrusted := flow.UntrustedLightCollection{
			Transactions: []flow.Identifier{unittest.IdentifierFixture()},
		}

		col := flow.NewLightCollection(untrusted)
		assert.NotNil(t, col)
		assert.Equal(t, untrusted.Transactions, col.Transactions)
	})

	t.Run("valid empty untrusted light collection", func(t *testing.T) {
		untrusted := flow.UntrustedLightCollection{}

		col := flow.NewLightCollection(untrusted)
		assert.NotNil(t, col)
		assert.Len(t, col.Transactions, 0)
	})
}
