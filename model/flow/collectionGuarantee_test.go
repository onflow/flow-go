package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCollectionGuaranteeID_Malleability confirms that the CollectionGuarantee struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestCollectionGuaranteeID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.CollectionGuaranteeFixture())
}

func TestNewCollectionGuarantee(t *testing.T) {
	t.Run("valid guarantee", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: unittest.IdentifierFixture(),
			ClusterChainID:   flow.Testnet,
			SignerIndices:    []byte{0, 1, 2},
			Signature:        unittest.SignatureFixture(),
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.NoError(t, err)
		assert.NotNil(t, guarantee)
	})

	t.Run("missing collection ID", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     flow.ZeroID,
			ReferenceBlockID: unittest.IdentifierFixture(),
			SignerIndices:    []byte{1},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "CollectionID")
	})

	t.Run("missing reference block ID", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: flow.ZeroID,
			SignerIndices:    []byte{1},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "ReferenceBlockID")
	})

	t.Run("missing signer indices", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: unittest.IdentifierFixture(),
			SignerIndices:    []byte{},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "SignerIndices")
	})
}
