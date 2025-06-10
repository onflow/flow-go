package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

// TestConvertCollection tests that converting a collection to a protobuf message results in the correct
// set of transaction IDs
func TestConvertCollection(t *testing.T) {
	t.Parallel()

	collection := unittest.CollectionFixture(5)
	txIDs := make([]flow.Identifier, 0, len(collection.Transactions))
	for _, tx := range collection.Transactions {
		txIDs = append(txIDs, tx.ID())
	}

	t.Run("convert collection to message", func(t *testing.T) {
		msg, err := convert.CollectionToMessage(&collection)
		require.NoError(t, err)

		assert.Len(t, msg.TransactionIds, len(txIDs))
		for i, txID := range txIDs {
			assert.Equal(t, txID[:], msg.TransactionIds[i])
		}
	})

	var msg *entities.Collection
	lightCollection := flow.LightCollection{Transactions: txIDs}

	t.Run("convert light collection to message", func(t *testing.T) {
		var err error
		msg, err = convert.LightCollectionToMessage(&lightCollection)
		require.NoError(t, err)

		assert.Len(t, msg.TransactionIds, len(txIDs))
		for i, txID := range txIDs {
			assert.Equal(t, txID[:], msg.TransactionIds[i])
		}
	})

	t.Run("convert message to light collection", func(t *testing.T) {
		lightColl, err := convert.MessageToLightCollection(msg)
		require.NoError(t, err)

		assert.Equal(t, len(txIDs), len(lightColl.Transactions))
		for _, txID := range lightColl.Transactions {
			assert.Equal(t, txID[:], txID[:])
		}
	})

	t.Run("convert full collection to message and then back to collection", func(t *testing.T) {
		msg, err := convert.FullCollectionToMessage(&collection)
		require.NoError(t, err)

		converted, err := convert.MessageToFullCollection(msg, flow.Testnet.Chain())
		require.NoError(t, err)

		assert.Equal(t, &collection, converted)
	})
}

// TestConvertCollectionGuarantee tests that converting a collection guarantee to and from a protobuf
// message results in the same collection guarantee
func TestConvertCollectionGuarantee(t *testing.T) {
	t.Parallel()

	guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollRef(unittest.IdentifierFixture()))

	msg := convert.CollectionGuaranteeToMessage(guarantee)
	converted, err := convert.MessageToCollectionGuarantee(msg)
	require.NoError(t, err)
	assert.Equal(t, guarantee, converted)
}

// TestConvertCollectionGuarantees tests that converting a collection guarantee to and from a protobuf
// message results in the same collection guarantee
func TestConvertCollectionGuarantees(t *testing.T) {
	t.Parallel()

	guarantees := unittest.CollectionGuaranteesFixture(5, unittest.WithCollRef(unittest.IdentifierFixture()))

	msg := convert.CollectionGuaranteesToMessages(guarantees)
	converted, err := convert.MessagesToCollectionGuarantees(msg)
	require.NoError(t, err)
	assert.Equal(t, guarantees, converted)
}

func TestNewCollectionGuarantee(t *testing.T) {
	t.Run("valid guarantee", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: unittest.IdentifierFixture(),
			ChainID:          flow.Testnet,
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
