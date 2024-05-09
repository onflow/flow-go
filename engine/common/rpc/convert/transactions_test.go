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

	tx := unittest.TransactionBodyFixture()
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
