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

// TestCollectionGuaranteeID_Malleability confirms that the CollectionGuarantee struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestCollectionGuaranteeID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.CollectionGuaranteeFixture())
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

	t.Run("nil transaction in list", func(t *testing.T) {
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
