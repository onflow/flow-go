package ingress

import (
	"context"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/mock"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	engine := mock.Engine{}

	h := handler{
		engine: &engine,
	}

	tx := unittest.TransactionFixture()
	engine.On("Submit", &tx).Return()

	res, err := h.SendTransaction(context.Background(), &observation.SendTransactionRequest{
		Transaction: convert.TransactionToMessage(tx),
	})
	require.NoError(t, err)

	// should submit the transaction to the engine
	engine.AssertCalled(t, "Submit", &tx)

	// should return the hash of the submitted transaction
	assert.Equal(t, tx.Hash(), crypto.Hash(res.Hash))
}
