package client_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/client"
	"github.com/dapperlabs/bamboo-node/client/mocks"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/utils/unittest"
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
