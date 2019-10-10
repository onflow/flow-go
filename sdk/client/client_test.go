package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/pkg/types/proto"
	"github.com/dapperlabs/flow-go/pkg/utils/unittest"
	"github.com/dapperlabs/flow-go/sdk/client"
	"github.com/dapperlabs/flow-go/sdk/client/mocks"
)

func TestSendTransaction(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	tx := unittest.TransactionFixture()

	// client should return non-error if RPC call succeeds
	mockRPC.EXPECT().
		SendTransaction(ctx, gomock.Any()).
		Return(&observe.SendTransactionResponse{Hash: tx.Hash()}, nil).
		Times(1)

	err := c.SendTransaction(ctx, tx)
	Expect(err).ToNot(HaveOccurred())

	// client should return error if RPC call fails
	mockRPC.EXPECT().
		SendTransaction(ctx, gomock.Any()).
		Return(nil, errors.New("dummy error")).
		Times(1)

	// error should be passed to user
	err = c.SendTransaction(ctx, tx)
	Expect(err).To(HaveOccurred())
}

func TestGetLatestBlock(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	res := &observe.GetLatestBlockResponse{
		Block: proto.BlockHeaderToMessage(unittest.BlockHeaderFixture()),
	}

	// client should return non-error if RPC call succeeds
	mockRPC.EXPECT().
		GetLatestBlock(ctx, gomock.Any()).
		Return(res, nil).
		Times(1)

	blockHeaderA, err := c.GetLatestBlock(ctx, true)
	Expect(err).ToNot(HaveOccurred())

	blockHeaderB := proto.MessageToBlockHeader(res.GetBlock())
	Expect(*blockHeaderA).To(Equal(blockHeaderB))

	// client should return error if RPC call fails
	mockRPC.EXPECT().
		GetLatestBlock(ctx, gomock.Any()).
		Return(nil, errors.New("dummy error")).
		Times(1)

	_, err = c.GetLatestBlock(ctx, true)
	Expect(err).To(HaveOccurred())
}

func TestCallScript(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	valueBytes, _ := json.Marshal(1)

	// client should return non-error if RPC call succeeds
	mockRPC.EXPECT().
		CallScript(ctx, gomock.Any()).
		Return(&observe.CallScriptResponse{Value: valueBytes}, nil).
		Times(1)

	value, err := c.CallScript(ctx, []byte("fun main(): Int { return 1 }"))
	Expect(err).ToNot(HaveOccurred())
	Expect(value).To(Equal(float64(1)))

	// client should return error if RPC call fails
	mockRPC.EXPECT().
		CallScript(ctx, gomock.Any()).
		Return(nil, errors.New("dummy error")).
		Times(1)

	// error should be passed to user
	_, err = c.CallScript(ctx, []byte("fun main(): Int { return 1 }"))
	Expect(err).To(HaveOccurred())

	// client should return error if value is empty
	mockRPC.EXPECT().
		CallScript(ctx, gomock.Any()).
		Return(&observe.CallScriptResponse{Value: []byte{}}, nil).
		Times(1)

	// error should be passed to user
	_, err = c.CallScript(ctx, []byte("fun main(): Int { return 1 }"))
	Expect(err).To(HaveOccurred())

	// client should return error if value is malformed
	mockRPC.EXPECT().
		CallScript(ctx, gomock.Any()).
		Return(&observe.CallScriptResponse{Value: []byte("asdfafa")}, nil).
		Times(1)

	// error should be passed to user
	_, err = c.CallScript(ctx, []byte("fun main(): Int { return 1 }"))
	Expect(err).To(HaveOccurred())
}

func TestGetEvents(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	// Set up a mock event response
	mockEvent := types.Event{
		ID: "Transfer",
		Values: map[string]interface{}{
			"to":   types.ZeroAddress(),
			"from": types.ZeroAddress(),
			"id":   1,
		},
	}
	events := []*types.Event{&mockEvent}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(events)
	Expect(err).ToNot(HaveOccurred())

	// Set up the mock to return a mocked event response
	mockRPC.EXPECT().
		GetEvents(ctx, gomock.Any()).
		Return(&observe.GetEventsResponse{EventsJson: buf.Bytes()}, nil).
		Times(1)

	// The client should pass the response to the client
	res, err := c.GetEvents(ctx, &types.EventQuery{})
	Expect(err).ToNot(HaveOccurred())
	Expect(len(res)).To(Equal(1))
	Expect(res[0].ID).To(Equal(mockEvent.ID))

	// Set up the mock to return an error
	mockRPC.EXPECT().
		GetEvents(ctx, gomock.Any()).
		Return(nil, fmt.Errorf("dummy error")).
		Times(1)

	// The client should pass along the error
	_, err = c.GetEvents(ctx, &types.EventQuery{})
	Expect(err).To(HaveOccurred())

	// Set up the mock to return a malformed eventsJSON response
	mockRPC.EXPECT().
		GetEvents(ctx, gomock.Any()).
		Return(&observe.GetEventsResponse{EventsJson: []byte{1,2,3,4}}, nil).
		Times(1)

	// The client should return an error because it should fail to decode
	_, err = c.GetEvents(ctx, &types.EventQuery{})
	Expect(err).To(HaveOccurred())
}
