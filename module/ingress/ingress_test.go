package ingress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow/protobuf/go/flow/access"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	engine := mocknetwork.Engine{}

	h := handler{
		chainID: flow.Testnet,
		engine:  &engine,
	}

	tx := unittest.TransactionBodyFixture()

	t.Run("should submit transaction to engine", func(t *testing.T) {
		engine.On("ProcessLocal", &tx).Return(nil).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		require.NoError(t, err)

		// should submit the transaction to the engine
		engine.AssertCalled(t, "ProcessLocal", &tx)

		// should return the fingerprint of the submitted transaction
		assert.Equal(t, tx.ID(), flow.HashToID(res.Id))
	})

	t.Run("should pass through error", func(t *testing.T) {
		expected := errors.New("error")
		engine.On("ProcessLocal", &tx).Return(expected).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		if assert.Error(t, err) {
			assert.Equal(t, expected, err)
		}

		// should submit the transaction to the engine
		engine.AssertCalled(t, "ProcessLocal", &tx)

		// should only return the error
		assert.Nil(t, res)
	})
}
