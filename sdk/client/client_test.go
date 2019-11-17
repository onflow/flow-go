package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/client/mocks"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestPing(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		mockRPC.EXPECT().
			Ping(ctx, gomock.Any()).
			Return(&observation.PingResponse{}, nil).
			Times(1)

		err := c.Ping(ctx)
		assert.Nil(t, err)
	})

	t.Run("ServerError", func(t *testing.T) {
		mockRPC.EXPECT().
			Ping(ctx, gomock.Any()).
			Return(nil, fmt.Errorf("fake error")).
			Times(1)

		err := c.Ping(ctx)
		assert.Error(t, err)
	})
}

func TestSendTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	tx := unittest.TransactionFixture()

	t.Run("Success", func(t *testing.T) {
		// client should return non-error if RPC call succeeds
		mockRPC.EXPECT().
			SendTransaction(ctx, gomock.Any()).
			Return(&observation.SendTransactionResponse{Hash: tx.Hash()}, nil).
			Times(1)

		err := c.SendTransaction(ctx, tx)
		assert.Nil(t, err)
	})

	t.Run("Server error", func(t *testing.T) {
		// client should return error if RPC call fails
		mockRPC.EXPECT().
			SendTransaction(ctx, gomock.Any()).
			Return(nil, errors.New("dummy error")).
			Times(1)

		// error should be passed to user
		err := c.SendTransaction(ctx, tx)
		assert.Error(t, err)
	})
}

func TestGetLatestBlock(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	res := &observation.GetLatestBlockResponse{
		Block: convert.BlockHeaderToMessage(unittest.BlockHeaderFixture()),
	}

	t.Run("Success", func(t *testing.T) {
		// client should return non-error if RPC call succeeds
		mockRPC.EXPECT().
			GetLatestBlock(ctx, gomock.Any()).
			Return(res, nil).
			Times(1)

		blockHeaderA, err := c.GetLatestBlock(ctx, true)
		assert.Nil(t, err)

		blockHeaderB := convert.MessageToBlockHeader(res.GetBlock())
		assert.Equal(t, *blockHeaderA, blockHeaderB)
	})

	t.Run("Server error", func(t *testing.T) {
		// client should return error if RPC call fails
		mockRPC.EXPECT().
			GetLatestBlock(ctx, gomock.Any()).
			Return(nil, errors.New("dummy error")).
			Times(1)

		_, err := c.GetLatestBlock(ctx, true)
		assert.Error(t, err)
	})
}

func TestExecuteScript(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	valueBytes, _ := json.Marshal(1)

	t.Run("Success", func(t *testing.T) {
		// client should return non-error if RPC call succeeds
		mockRPC.EXPECT().
			ExecuteScript(ctx, gomock.Any()).
			Return(&observation.ExecuteScriptResponse{Value: valueBytes}, nil).
			Times(1)

		value, err := c.ExecuteScript(ctx, []byte("fun main(): Int { return 1 }"))
		assert.Nil(t, err)
		assert.Equal(t, value, float64(1))
	})

	t.Run("Server error", func(t *testing.T) {
		// client should return error if RPC call fails
		mockRPC.EXPECT().
			ExecuteScript(ctx, gomock.Any()).
			Return(nil, errors.New("dummy error")).
			Times(1)

		// error should be passed to user
		_, err := c.ExecuteScript(ctx, []byte("fun main(): Int { return 1 }"))
		assert.Error(t, err)
	})

	t.Run("Error - empty return value", func(t *testing.T) {
		// client should return error if value is empty
		mockRPC.EXPECT().
			ExecuteScript(ctx, gomock.Any()).
			Return(&observation.ExecuteScriptResponse{Value: []byte{}}, nil).
			Times(1)

		// error should be passed to user
		_, err := c.ExecuteScript(ctx, []byte("fun main(): Int { return 1 }"))
		assert.Error(t, err)
	})

	t.Run("Error - malformed return value", func(t *testing.T) {
		// client should return error if value is malformed
		mockRPC.EXPECT().
			ExecuteScript(ctx, gomock.Any()).
			Return(&observation.ExecuteScriptResponse{Value: []byte("asdfafa")}, nil).
			Times(1)

		// error should be passed to user
		_, err := c.ExecuteScript(ctx, []byte("fun main(): Int { return 1 }"))
		assert.Error(t, err)
	})
}

func TestGetTransaction(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	tx := unittest.TransactionFixture()

	events := []flow.Event{unittest.EventFixture()}

	eventMessages := make([]*entities.Event, len(events))
	for i, event := range events {
		eventMessages[i] = convert.EventToMessage(event)
	}

	t.Run("Success", func(t *testing.T) {
		mockRPC.EXPECT().
			GetTransaction(ctx, gomock.Any()).
			Return(&observation.GetTransactionResponse{
				Transaction: convert.TransactionToMessage(tx),
				Events:      eventMessages,
			}, nil).
			Times(1)

		res, err := c.GetTransaction(ctx, crypto.Hash{})
		assert.Nil(t, err)
		assert.Len(t, res.Events, 1)
		assert.Equal(t, events[0].Type, res.Events[0].Type)
	})

	t.Run("Server error", func(t *testing.T) {
		mockRPC.EXPECT().
			GetTransaction(ctx, gomock.Any()).
			Return(nil, fmt.Errorf("dummy error")).
			Times(1)

		// The client should pass along the error
		_, err := c.GetTransaction(ctx, crypto.Hash{})
		assert.Error(t, err)
	})
}

func TestGetEvents(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	// declare event type used for decoding event payloads
	mockEventType := types.Event{
		Identifier: "Transfer",
		FieldTypes: []types.EventField{
			{
				Identifier: "to",
				Type:       types.Address{},
			},
			{
				Identifier: "from",
				Type:       types.Address{},
			},
			{
				Identifier: "amount",
				Type:       types.Int{},
			},
		},
	}

	to := values.Address(flow.ZeroAddress)
	from := values.Address(flow.ZeroAddress)
	amount := values.Int(42)

	mockEventValue := values.Event{
		Identifier: "Transfer",
		Fields:     []values.Value{to, from, amount},
	}

	// encode event payload from mock value
	eventPayload, _ := encoding.Encode(mockEventValue)

	// Set up a mock event response
	mockEvent := flow.Event{
		Type:    "Transfer",
		Payload: eventPayload,
	}

	t.Run("Success", func(t *testing.T) {
		// Set up the mock to return a mocked event response
		mockRes := &observation.GetEventsResponse{Events: []*entities.Event{
			convert.EventToMessage(mockEvent),
		}}

		mockRPC.EXPECT().
			GetEvents(ctx, gomock.Any()).
			Return(mockRes, nil).
			Times(1)

		// The client should pass the response to the client
		events, err := c.GetEvents(ctx, client.EventQuery{})
		assert.Nil(t, err)
		require.Len(t, events, 1)

		actualEvent := events[0]

		value, err := encoding.Decode(mockEventType, actualEvent.Payload)
		eventValue := value.(values.Event)

		assert.Equal(t, actualEvent.Type, mockEvent.Type)
		assert.Equal(t, to, eventValue.Fields[0])
		assert.Equal(t, from, eventValue.Fields[1])
		assert.Equal(t, amount, eventValue.Fields[2])
	})

	t.Run("Server error", func(t *testing.T) {
		// Set up the mock to return an error
		mockRPC.EXPECT().
			GetEvents(ctx, gomock.Any()).
			Return(nil, fmt.Errorf("dummy error")).
			Times(1)

		// The client should pass along the error
		_, err := c.GetEvents(ctx, client.EventQuery{})
		assert.Error(t, err)
	})
}
