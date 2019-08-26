package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/utils/unittest"
	"github.com/dapperlabs/bamboo-node/sdk/client"
	"github.com/dapperlabs/bamboo-node/sdk/client/mocks"
)

func TestSendTransaction(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	tx := unittest.SignedTransactionFixture()

	// client should return non-error if RPC call succeeds
	mockRPC.EXPECT().
		SendTransaction(ctx, gomock.Any()).
		Return(&observe.SendTransactionResponse{Hash: tx.Hash().Bytes()}, nil).
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
