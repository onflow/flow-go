package ingress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
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

	t.Run("should submit transaction to engine", func(t *testing.T) {
		engine.On("Submit", &tx).Return(nil).Once()

		res, err := h.SendTransaction(context.Background(), &observation.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		require.NoError(t, err)

		// should submit the transaction to the engine
		engine.AssertCalled(t, "Submit", &tx)

		// should return the fingerprint of the submitted transaction
		assert.Equal(t, tx.Fingerprint(), model.Fingerprint(res.Hash))
	})

	t.Run("should pass through error", func(t *testing.T) {
		expected := errors.New("error")
		engine.On("Submit", &tx).Return(expected).Once()

		res, err := h.SendTransaction(context.Background(), &observation.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		if assert.Error(t, err) {
			assert.Equal(t, expected, err)
		}

		// should submit the transaction to the engine
		engine.AssertCalled(t, "Submit", &tx)

		// should only return the error
		assert.Nil(t, res)
	})
}
