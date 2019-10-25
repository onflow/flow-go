package server_test

import (
	"context"
	"encoding/json"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
	"github.com/dapperlabs/flow-go/sdk/emulator/server"
)

func TestPing(t *testing.T) {
	ctx := context.Background()
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)
	server := server.NewBackend(b, events.NewMemStore(), log.New())

	res, err := server.Ping(ctx, &observe.PingRequest{})
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
		mintID     = "Mint()"
		transferID = "Transfer(to: Address)"
		toAddress  = types.HexToAddress("1234567890123456789012345678901234567890")

		ev1 = types.Event{
			ID:     mintID,
			Values: map[string]interface{}{},
		}
		ev2 = types.Event{
			ID: transferID,
			Values: map[string]interface{}{
				"to": toAddress.String(),
			},
		}
	)

	eventStore.Add(ctx, 1, ev1)
	eventStore.Add(ctx, 3, ev2)

	t.Run("should return error for invalid query", func(t *testing.T) {
		// End block cannot be less than start block
		_, err := server.GetEvents(ctx, &observe.GetEventsRequest{
			StartBlock: 2,
			EndBlock:   1,
		})
		assert.Error(t, err)
	})
	t.Run("should return empty list when there are no results", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observe.GetEventsRequest{
			StartBlock: 2,
			EndBlock:   2,
		})
		assert.Nil(t, err)
		var events []*types.Event
		err = json.Unmarshal(res.GetEventsJson(), &events)
		assert.Nil(t, err)
		assert.Len(t, events, 0)
	})
	t.Run("should filter by ID", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observe.GetEventsRequest{
			EventId:    transferID,
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []*types.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		assert.Len(t, resEvents, 1)
		assert.Equal(t, resEvents[0].ID, transferID)
		assert.Equal(t, resEvents[0].Values["to"], toAddress.String())
	})
	t.Run("should not filter by ID when omitted", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observe.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []*types.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		// Should get both events
		assert.Len(t, resEvents, 2)
	})
	t.Run("should preserve event ordering", func(t *testing.T) {
		res, err := server.GetEvents(ctx, &observe.GetEventsRequest{
			StartBlock: 1,
			EndBlock:   3,
		})
		assert.Nil(t, err)
		var resEvents []*types.Event
		err = json.Unmarshal(res.GetEventsJson(), &resEvents)
		assert.Nil(t, err)
		assert.Len(t, resEvents, 2)
		// Mint event first, then Transfer
		assert.Equal(t, resEvents[0].ID, mintID)
		assert.Equal(t, resEvents[1].ID, transferID)
	})
}
