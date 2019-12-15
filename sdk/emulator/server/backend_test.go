package server_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/execution"
	"github.com/dapperlabs/flow-go/sdk/emulator/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
	etypes "github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	b, err := emulator.NewEmulatedBlockchain()
	require.NoError(t, err)
	server := server.NewBackend(b, log.New())

	res, err := server.Ping(ctx, &observation.PingRequest{})
	assert.NoError(t, err)
	assert.Equal(t, res.GetAddress(), []byte("pong!"))
}

func TestBackend(t *testing.T) {

	//wrapper which manages mock lifecycle
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockEmulatedBlockchainAPI(mockCtrl)

			backend := server.NewBackend(
				api,
				log.New(),
			)

			sut(t, backend, api)
		}
	}

	t.Run("ExecuteScript", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		executionScriptRequest := observation.ExecuteScriptRequest{
			Script: sampleScriptText,
		}

		api.EXPECT().
			ExecuteScript(sampleScriptText).Return(values.NewInt(2137), nil, nil).
			Times(1)

		response, err := backend.ExecuteScript(context.Background(), &executionScriptRequest)
		assert.NoError(t, err)

		value, err := encoding.Decode(types.Int{}, response.GetValue())
		assert.NoError(t, err)

		assert.Equal(t, values.NewInt(2137), value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		account := unittest.AccountFixture()

		api.EXPECT().
			GetAccount(account.Address).
			Return(&account, nil).
			Times(1)

		request := observation.GetAccountRequest{
			Address: account.Address.Bytes(),
		}
		response, err := backend.GetAccount(context.Background(), &request)

		assert.NoError(t, err)

		assert.Equal(t, account.Address.Bytes(), response.Account.Address)
		assert.Equal(t, account.Balance, response.Account.Balance)
	}))

	t.Run("GetEvents fails with wrong block numbers", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]flow.Event{}, nil).
			Times(0)

		response, err := backend.GetEvents(context.Background(), &observation.GetEventsRequest{
			Type:       "SomeEvents",
			StartBlock: 37,
			EndBlock:   21,
		})

		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
	}))

	t.Run("GetEvents fails if blockchain returns error", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {
		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("dummy")).
			Times(1)

		_, err := backend.GetEvents(context.Background(), &observation.GetEventsRequest{})

		if assert.Error(t, err) {
			grpcError, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, grpcError.Code(), codes.Internal)
		}
	}))

	t.Run("GetEvents", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		eventType := "SomeEvents"

		eventsToReturn := []flow.Event{
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 0
			}),
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 1
			}),
		}

		api.EXPECT().
			GetEvents(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(eventsToReturn, nil).
			Times(1)

		var startBlock uint64 = 21
		var endBlock uint64 = 37

		response, err := backend.GetEvents(context.Background(), &observation.GetEventsRequest{
			Type:       eventType,
			StartBlock: startBlock,
			EndBlock:   endBlock,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		resEvents := response.GetEvents()

		assert.Len(t, resEvents, 2)
		assert.EqualValues(t, 0, resEvents[0].GetIndex())
		assert.EqualValues(t, 1, resEvents[1].GetIndex())
	}))

	t.Run("GetLatestBlock", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {
		block := etypes.Block{
			Number:            11,
			PreviousBlockHash: nil,
			TransactionHashes: nil,
		}

		api.EXPECT().
			GetLatestBlock().
			Return(&block, nil).
			Times(1)

		response, err := backend.GetLatestBlock(context.Background(), &observation.GetLatestBlockRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, block.Number, response.Block.Number)
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		bytes := []byte{2, 2, 2, 2}

		api.EXPECT().
			GetTransaction(gomock.Any()).
			Return(nil, &emulator.ErrTransactionNotFound{}).
			Times(1)

		response, err := backend.GetTransaction(context.Background(), &observation.GetTransactionRequest{
			Hash: bytes,
		})

		assert.Error(t, err)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.NotFound, grpcError.Code())

		assert.Nil(t, response)
	}))

	t.Run("GetTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		txEventType := "TxEvent"

		tx := unittest.TransactionFixture(func(t *flow.Transaction) {
			t.Status = flow.TransactionSealed
		})

		txHash := tx.Hash()

		txEvents := []flow.Event{
			unittest.EventFixture(func(e *flow.Event) {
				e.TxHash = txHash
				e.Type = txEventType
			}),
		}

		tx.Events = txEvents

		api.EXPECT().
			GetTransaction(gomock.Eq(txHash)).
			Return(&tx, nil).
			Times(1)

		response, err := backend.GetTransaction(context.Background(), &observation.GetTransactionRequest{
			Hash: txHash,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, tx.Nonce, response.Transaction.Nonce)

		resEvents := response.GetEvents()

		require.Len(t, resEvents, 1)

		event := convert.MessageToEvent(resEvents[0])

		assert.Equal(t, txHash, event.TxHash)
		assert.Equal(t, txEventType, event.Type)
	}))

	t.Run("Ping", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		response, err := backend.Ping(context.Background(), &observation.PingRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)
	}))

	t.Run("SendTransaction with nil tx", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		response, err := backend.SendTransaction(context.Background(), &observation.SendTransactionRequest{
			Transaction: nil,
		})

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

	t.Run("SendTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		var capturedTx flow.Transaction

		api.EXPECT().
			AddTransaction(gomock.Any()).
			DoAndReturn(func(tx flow.Transaction) (execution.TransactionResult, error) {
				capturedTx = tx
				return execution.TransactionResult{}, nil
			}).Times(1)

		requestTx := observation.SendTransactionRequest{
			Transaction: &entities.Transaction{
				Script:             nil,
				ReferenceBlockHash: nil,
				Nonce:              2137,
				ComputeLimit:       7,
				PayerAccount:       nil,
				ScriptAccounts: [][]byte{
					nil,
					{1, 2, 3, 4},
				},
				Signatures: []*entities.AccountSignature{
					{
						Account:   []byte{2, 2, 2, 2},
						Signature: []byte{4, 4, 4, 4},
					}, nil,
				},
				Status: 0,
			},
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, requestTx.Transaction.Nonce, capturedTx.Nonce)
		assert.Len(t, capturedTx.ScriptAccounts, 2)
		assert.Len(t, capturedTx.Signatures, 2)

		assert.True(t, capturedTx.Hash().Equal(response.Hash))
	}))

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI) {

		api.EXPECT().
			AddTransaction(gomock.Any()).
			Return(execution.TransactionResult{}, &emulator.ErrInvalidSignaturePublicKey{}).Times(1)

		requestTx := observation.SendTransactionRequest{
			Transaction: &entities.Transaction{
				Script:             nil,
				ReferenceBlockHash: nil,
				Nonce:              7,
				ComputeLimit:       2137,
				PayerAccount:       nil,
				ScriptAccounts:     nil,
				Signatures:         nil,
				Status:             0,
			},
		}
		response, err := backend.SendTransaction(context.Background(), &requestTx)

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

}
