package server_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
	"github.com/dapperlabs/flow-go/sdk/emulator/mocks"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
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
	withMocks := func(sut func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi)) func(t *testing.T) {
		return func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			api := mocks.NewMockEmulatedBlockchainApi(mockCtrl)

			backend := server.NewBackend(
				api,
				events.NewMemStore(),
				log.New(),
			)

			sut(t, backend, api)
		}
	}

	t.Run("ExecuteScript", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi) {
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

	t.Run("GetAccount", withMocks(func(t *testing.T, backend *server.Backend, api *mocks.MockEmulatedBlockchainApi) {

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

}
