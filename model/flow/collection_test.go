package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLightCollectionFingerprint(t *testing.T) {
	col := unittest.CollectionFixture(2)
	colID := col.ID()
	data := fingerprint.Fingerprint(col.Light())
	var decoded flow.LightCollection
	rlp.NewMarshaler().MustUnmarshal(data, &decoded)
	decodedID := decoded.ID()
	assert.Equal(t, colID, decodedID)
	assert.Equal(t, col.Light(), decoded)
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
		assert.Error(t, err)
		assert.Nil(t, col)
		assert.Contains(t, err.Error(), "transactions must not be nil")
	})
}
