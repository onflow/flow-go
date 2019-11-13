package server_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
	event_mocks "github.com/dapperlabs/flow-go/sdk/emulator/events/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"
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
		mintType     = "Mint()"
		transferType = "Transfer(to: Address)"
		toAddress    = flow.HexToAddress("1234567890123456789012345678901234567890")

		ev1 = flow.Event{
			Type:   mintType,
			Values: map[string]interface{}{},
		}
		ev2 = flow.Event{
			Type: transferType,
			Values: map[string]interface{}{
				"to": toAddress.String(),
			},
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
		var events []flow.Event
		err = json.Unmarshal(res.GetEventsJson(), &events)
		assert.Nil(t, err)
		assert.Len(t, events, 0)
	})

	t.Run("should filter by ID", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			Type:       transferType,
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []flow.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		assert.Len(t, resEvents, 1)
		assert.Equal(t, resEvents[0].Type, transferType)
		assert.Equal(t, resEvents[0].Values["to"], toAddress.String())
	})

	t.Run("should not filter by ID when omitted", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []flow.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		// Should get both events
		assert.Len(t, resEvents, 2)
	})

	t.Run("should preserve event ordering", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observation.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []flow.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		assert.Len(t, resEvents, 2)
		// Mint event first, then Transfer
		assert.Equal(t, resEvents[0].Type, mintType)
		assert.Equal(t, resEvents[1].Type, transferType)
	})
}

func TestBackend(t *testing.T) {

	//wrapper which manages mock lifecycle
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockEmulatedBlockchainApi(mockCtrl)
			events := event_mocks.NewMockStore(mockCtrl)

			backend := server.NewBackend(
				api,
				events,
				log.New(),
			)

			sut(t, backend, api, events)
		}
	}

	t.Run("ExecuteScript", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {
		sampleScriptText := []byte("hey I'm so totally uninterpretable script text with unicode ć, ń, ó, ś, ź")
		executionScriptRequest := observation.ExecuteScriptRequest{
			Script: sampleScriptText,
		}

		api.EXPECT().
			ExecuteScript(sampleScriptText).Return(big.NewInt(2137), nil).
			Times(1)

		response, err := backend.ExecuteScript(context.TODO(), &executionScriptRequest)

		assert.Nil(t, err)

		//TODO likely to be refactored with a proper serialization/ABI implemented
		assert.Equal(t, "*big.Int", response.Type)
		assert.Equal(t, []uint8("2137"), response.Value)
	}))

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		initialAddressBytes := []byte{0, 1, 2, 3, 4, 5}
		address := flow.BytesToAddress(initialAddressBytes)

		account := flow.Account{
			Address: address,
			Balance: 2137,
			Code:    nil,
			Keys:    nil,
		}

		api.EXPECT().
			GetAccount(address).
			Return(&account, nil).
			Times(1)

		request := observation.GetAccountRequest{
			Address: initialAddressBytes,
		}
		response, err := backend.GetAccount(context.TODO(), &request)

		assert.Nil(t, err)

		assert.Equal(t, address.Bytes(), response.Account.Address)
		assert.Equal(t, uint64(2137), response.Account.Balance)
	}))

	t.Run("GetEvents fails with wrong block numbers", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		events.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

		response, err := backend.GetEvents(context.TODO(), &observation.GetEventsRequest{
			Type:       "SomeEvents",
			StartBlock: 37,
			EndBlock:   21,
		})

		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)

		assert.True(t, ok)
		assert.Equal(t, grpcError.Code(), codes.InvalidArgument)
	}))

	t.Run("GetEvents", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		eventType := "SomeEvents"

		eventsToReturn := []flow.Event{
			{
				TxHash: nil,
				Type:   eventType,
				Values: nil,
				Index:  0,
			},
			{
				TxHash: nil,
				Type:   eventType,
				Values: nil,
				Index:  1,
			}}

		events.EXPECT().Query(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(eventsToReturn, nil).Times(1)

		startBlock := 21
		endBlock := 37

		response, err := backend.GetEvents(context.TODO(), &observation.GetEventsRequest{
			Type:       eventType,
			StartBlock: uint64(startBlock),
			EndBlock:   uint64(endBlock),
		})

		assert.Nil(t, err)

		assert.NotNil(t, response)

		var decodedEvents []flow.Event

		err = json.NewDecoder(bytes.NewReader(response.EventsJson)).Decode(&decodedEvents)
		assert.NoError(t, err)

		assert.Len(t, decodedEvents, 2)
		assert.Equal(t, uint(0), decodedEvents[0].Index)
		assert.Equal(t, uint(1), decodedEvents[1].Index)
	}))

	t.Run("GetLatestBlock", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		blockTimestamp := time.Time{}
		block := types.Block{
			Number:            11,
			Timestamp:         blockTimestamp,
			PreviousBlockHash: nil,
			TransactionHashes: nil,
		}

		api.EXPECT().
			GetLatestBlock().
			Return(&block).
			Times(1)

		response, err := backend.GetLatestBlock(context.TODO(), &observation.GetLatestBlockRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, block.Number, response.Block.Number)
	}))

	t.Run("GetTransaction tx does not exists", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		bytes := []byte{2, 2, 2, 2}

		api.EXPECT().
			GetTransaction(gomock.Any()).
			Return(nil, &emulator.ErrTransactionNotFound{}).
			Times(1)

		response, err := backend.GetTransaction(context.TODO(), &observation.GetTransactionRequest{
			Hash: bytes,
		})

		assert.Error(t, err)

		grpcError, ok := status.FromError(err)
		assert.True(t, ok)

		assert.Equal(t, codes.NotFound, grpcError.Code())

		assert.Nil(t, response)
	}))

	t.Run("GetTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		txEventType := "TxEvent"

		tx := flow.Transaction{
			Script:             nil,
			ReferenceBlockHash: nil,
			Nonce:              2137,
			ComputeLimit:       2,
			PayerAccount:       flow.Address{},
			ScriptAccounts:     nil,
			Signatures:         nil,
			Status:             flow.TransactionSealed,
			Events:             nil,
		}

		txHash := tx.Hash()

		txEvents := []flow.Event{
			{
				TxHash: txHash,
				Type:   txEventType,
				Values: nil,
				Index:  0,
			},
		}

		tx.Events = txEvents

		api.EXPECT().
			GetTransaction(gomock.Eq(txHash)).
			Return(&tx, nil).
			Times(1)

		response, err := backend.GetTransaction(context.TODO(), &observation.GetTransactionRequest{
			Hash: txHash,
		})

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, tx.Nonce, response.Transaction.Nonce)

		var decodedEvents []flow.Event

		err = json.NewDecoder(bytes.NewReader(response.EventsJson)).Decode(&decodedEvents)
		assert.NoError(t, err)

		assert.Len(t, decodedEvents, 1)
		assert.Equal(t, txHash, decodedEvents[0].TxHash)
		assert.Equal(t, txEventType, decodedEvents[0].Type)
	}))

	t.Run("Ping", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		response, err := backend.Ping(context.TODO(), &observation.PingRequest{})

		assert.NoError(t, err)
		assert.NotNil(t, response)
	}))

	t.Run("SendTransaction with nil tx", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

		response, err := backend.SendTransaction(context.TODO(), &observation.SendTransactionRequest{
			Transaction: nil,
		})

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		assert.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

	t.Run("SendTransaction", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

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
		response, err := backend.SendTransaction(context.TODO(), &requestTx)

		assert.NoError(t, err)
		assert.NotNil(t, response)

		assert.Equal(t, requestTx.Transaction.Nonce, capturedTx.Nonce)
		assert.Len(t, capturedTx.ScriptAccounts, 2)
		assert.Len(t, capturedTx.Signatures, 2)

		assert.True(t, capturedTx.Hash().Equal(response.Hash))
	}))

	t.Run("SendTransaction which errors while processing", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi, events *event_mocks.MockStore) {

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
		response, err := backend.SendTransaction(context.TODO(), &requestTx)

		assert.Error(t, err)
		assert.Nil(t, response)

		grpcError, ok := status.FromError(err)
		assert.True(t, ok)

		assert.Equal(t, codes.InvalidArgument, grpcError.Code())

	}))

}
