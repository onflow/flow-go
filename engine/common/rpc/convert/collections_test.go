package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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

	t.Run("convert light collection to message", func(t *testing.T) {
		lightCollection := flow.LightCollection{Transactions: txIDs}

		msg, err := convert.LightCollectionToMessage(&lightCollection)
		require.NoError(t, err)

		assert.Len(t, msg.TransactionIds, len(txIDs))
		for i, txID := range txIDs {
			assert.Equal(t, txID[:], msg.TransactionIds[i])
		}
	})
}

// TestConvertCollectionGuarantee tests that converting a collection guarantee to and from a protobuf
// message results in the same collection guarantee
func TestConvertCollectionGuarantee(t *testing.T) {
	t.Parallel()

	guarantee := unittest.CollectionGuaranteeFixture(unittest.WithCollRef(unittest.IdentifierFixture()))

	msg := convert.CollectionGuaranteeToMessage(guarantee)
	converted := convert.MessageToCollectionGuarantee(msg)

	assert.Equal(t, guarantee, converted)
}

// TestConvertCollectionGuarantees tests that converting a collection guarantee to and from a protobuf
// message results in the same collection guarantee
func TestConvertCollectionGuarantees(t *testing.T) {
	t.Parallel()

	guarantees := unittest.CollectionGuaranteesFixture(5, unittest.WithCollRef(unittest.IdentifierFixture()))

	msg := convert.CollectionGuaranteesToMessages(guarantees)
	converted := convert.MessagesToCollectionGuarantees(msg)

	assert.Equal(t, guarantees, converted)
}
