package rpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow/protobuf/go/flow/access"

	rpcmock "github.com/onflow/flow-go/engine/collection/rpc/mock"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	backend := new(rpcmock.Backend)

	h := handler{
		chainID: flow.Testnet,
		backend: backend,
	}

	tx := unittest.TransactionBodyFixture()

	t.Run("should submit transaction to engine", func(t *testing.T) {
		backend.On("ProcessTransaction", &tx).Return(nil).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		require.NoError(t, err)

		// should submit the transaction to the engine
		backend.AssertCalled(t, "ProcessTransaction", &tx)

		// should return the fingerprint of the submitted transaction
		assert.Equal(t, tx.ID(), flow.HashToID(res.Id))
	})

	t.Run("should pass through error", func(t *testing.T) {
		expected := errors.New("error")
		backend.On("ProcessTransaction", &tx).Return(expected).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		if assert.Error(t, err) {
			assert.Equal(t, expected, err)
		}

		// should submit the transaction to the engine
		backend.AssertCalled(t, "ProcessTransaction", &tx)

		// should only return the error
		assert.Nil(t, res)
	})
}
