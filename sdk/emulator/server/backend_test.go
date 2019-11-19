package server_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
	event_mocks "github.com/dapperlabs/flow-go/sdk/emulator/events/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
	etypes "github.com/dapperlabs/flow-go/sdk/emulator/types"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)
	server := server.NewBackend(b, events.NewMemStore(), log.New())

	res, err := server.Ping(ctx, &observation.PingRequest{})
	assert.Nil(t, err)
	assert.Equal(t, res.GetAddress(), []byte("pong!"))
}

func TestGetEvents(t *testing.T) {
	ctx := context.Background()
	eventStore := events.NewMemStore()
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)
	server := server.NewBackend(b, eventStore, log.New())

	// Add some events
	var (
		mintType     = "Mint"
		transferType = "Transfer"
		toAddress    = values.Address(flow.HexToAddress("1234567890123456789012345678901234567890"))

		transferEventType types.Type = types.Event{
			Identifier: transferType,
			FieldTypes: []types.EventField{
				{
					Identifier: "to",
					Type:       types.Address{},
				},
			},
		}

		ev1 = flow.Event{
			Type:    mintType,
			Payload: []byte{},
		}

		transferPayload, _ = encoding.Encode(values.Event{
			Identifier: transferType,
			Fields:     []values.Value{toAddress},
		})

		ev2 = flow.Event{
			Type:    transferType,
			Payload: transferPayload,
		}
	)

	err := eventStore.Add(ctx, 1, ev1)
	require.Nil(t, err)

	err = eventStore.Add(ctx, 3, ev2)
	require.Nil(t, err)

	t.Run("should return error for invalid query", func(t *testing.T) {
		// End block cannot be less than start block
		_, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 2,
			EndBlock:   1,
		})
		assert.Error(t, err)
	})

	t.Run("should return empty list when there are no results", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 2,
			EndBlock:   2,
		})
		assert.Nil(t, err)
		assert.Len(t, res.GetEvents(), 0)
	})

	t.Run("should filter by ID", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			Type:       transferType,
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)

		resEvents := res.GetEvents()

		assert.Len(t, resEvents, 1)
		assert.Equal(t, transferType, resEvents[0].GetType())

		value, err := encoding.Decode(transferEventType, resEvents[0].GetPayload())
		require.Nil(t, err)

		event := value.(values.Event)
		assert.Equal(t, toAddress, event.Fields[0])
	})

	t.Run("should not filter by ID when omitted", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)

		// Should get both events
		assert.Len(t, res.GetEvents(), 2)
	})

	t.Run("should preserve event ordering", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)

		resEvents := res.GetEvents()
		assert.Len(t, resEvents, 2)
		// Mint event first, then Transfer
		assert.Equal(t, mintType, resEvents[0].GetType())
		assert.Equal(t, transferType, resEvents[1].GetType())
	})
}

func TestBackend(t *testing.T) {

	//wrapper which manages mock lifecycle
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockEmulatedBlockchainAPI(mockCtrl)
			events := event_mocks.NewMockStore(mockCtrl)

			backend := server.NewBackend(
				api,
				events,
				log.New(),
			)

			sut(t, backend, api, events)
		}
	}

	t.Run("ExecuteScript", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		executionScriptRequest := observation.ExecuteScriptRequest{
			Script: sampleScriptText,
		}

		api.EXPECT().
			ExecuteScript(sampleScriptText).Return(big.NewInt(2137), nil).
			Times(1)

		response, err := backend.ExecuteScript(context.Background(), &executionScriptRequest)

		assert.Nil(t, err)

		// TODO likely to be refactored with a proper serialization/ABI implemented
		assert.Equal(t, "*big.Int", response.Type)
		assert.Equal(t, []uint8("2137"), response.Value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		account := unittest.AccountFixture()

		api.EXPECT().
			GetAccount(account.Address).
			Return(&account, nil).
			Times(1)

		request := observation.GetAccountRequest{
			Address: account.Address.Bytes(),
		}
		response, err := backend.GetAccount(context.Background(), &request)

		assert.Nil(t, err)

		assert.Equal(t, account.Address.Bytes(), response.Account.Address)
		assert.Equal(t, account.Balance, response.Account.Balance)
	}))

	t.Run("GetEvents fails with wrong block numbers", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		events.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

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

	t.Run("GetEvents", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		eventType := "SomeEvents"

		eventsToReturn := []flow.Event{
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 0
			}),
			unittest.EventFixture(func(e *flow.Event) {
				e.Index = 1
			}),
		}

		events.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(eventsToReturn, nil).Times(1)

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

	t.Run("GetLatestBlock", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		blockTimestamp := time.Time{}
		block := etypes.Block{
			Number:            11,
			Timestamp:         blockTimestamp,
			PreviousBlockHash: nil,
			TransactionHashes: nil,
		}

		api.EXPECT().
			GetLatestBlock().
			Return(&block).
			Times(1)

		response, err := backend.GetLatestBlock(context.Background(), &observation.GetLatestBlockRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, block.Number, response.Block.Number)
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

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

	t.Run("GetTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

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

	t.Run("Ping", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		response, err := backend.Ping(context.Background(), &observation.PingRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)
	}))

	t.Run("SendTransaction with nil tx", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		response, err := backend.SendTransaction(context.Background(), &observation.SendTransactionRequest{
			Transaction: nil,
		})

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		require.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

	t.Run("SendTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		var capturedTx flow.Transaction

		api.EXPECT().
			SubmitTransaction(gomock.Any()).
			DoAndReturn(func(tx flow.Transaction) error {
				capturedTx = tx
				return nil
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

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainAPI, events *event_mocks.MockStore) {

		api.EXPECT().
			SubmitTransaction(gomock.Any()).
			Return(&emulator.ErrInvalidSignaturePublicKey{}).Times(1)

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
