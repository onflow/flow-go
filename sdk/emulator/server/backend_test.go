package server_test

import (
	"context"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encode"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
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

		transferPayload, _ = encode.Encode(values.Event{
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
		assert.Equal(t, transferType, resEvents[0].Type)

		value, err := encode.Decode(transferEventType, resEvents[0].Payload)
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
		assert.Equal(t, resEvents[0].Type, mintType)
		assert.Equal(t, resEvents[1].Type, transferType)
	})
}
