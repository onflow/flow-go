package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertTransaction(t *testing.T) {
	t.Parallel()

	tx := unittest.TransactionFixture()
	arg, err := jsoncdc.Encode(cadence.NewAddress(unittest.AddressFixture()))
	require.NoError(t, err)

	// add fields not included in the fixture
	tx.Arguments = append(tx.Arguments, arg)
	tx.EnvelopeSignatures = append(tx.EnvelopeSignatures, unittest.TransactionSignatureFixture())

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg, flow.Testnet.Chain())
	require.NoError(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

// TestConvertTransactionsToMessages tests converting multiple flow.TransactionBody to protobuf messages.
func TestConvertTransactionsToMessages(t *testing.T) {
	t.Parallel()

	// Create multiple transactions with varying fields
	transactions := make([]*flow.TransactionBody, 6)
	for i := 0; i < len(transactions); i++ {
		tx := unittest.TransactionFixture()

		// Add some variation to exercise different code paths
		if i%2 == 0 {
			arg, err := jsoncdc.Encode(cadence.NewAddress(unittest.AddressFixture()))
			require.NoError(t, err)
			tx.Arguments = append(tx.Arguments, arg)
		}

		if i%3 == 0 {
			tx.EnvelopeSignatures = append(tx.EnvelopeSignatures, unittest.TransactionSignatureFixture())
		}

		transactions[i] = &tx
	}

	messages := convert.TransactionsToMessages(transactions)
	require.Len(t, messages, len(transactions))

	chain := flow.Testnet.Chain()
	for i, msg := range messages {
		converted, err := convert.MessageToTransaction(msg, chain)
		require.NoError(t, err)
		assert.Equal(t, *transactions[i], converted)
		assert.Equal(t, transactions[i].ID(), converted.ID())
	}
}
