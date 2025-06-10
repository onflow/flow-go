package flow_test

import (
	"testing"

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
	t.Skip("TODO: postponing malleability ID changes to mainnet27")
	unittest.RequireEntityNonMalleable(t, unittest.CollectionGuaranteeFixture())
}
