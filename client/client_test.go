package client_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/client"
	"github.com/dapperlabs/bamboo-node/client/mocks"
	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/utils/unittest"
)

var defaultAccount, _ = client.LoadAccount(strings.NewReader(`
	{	
		"account": "0xdd2781f4c51bccdbe23e4d398b8a82261f585c278dbb4b84989fea70e76723a9",
		"seed": "elephant ears"
	}
`))

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

func TestCreateAccount(t *testing.T) {
	RegisterTestingT(t)

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockRPC := mocks.NewMockRPCClient(mockCtrl)

	c := client.NewFromRPCClient(mockRPC)
	ctx := context.Background()

	keyPair, _ := crypto.GenerateKeyPair("elephant ears")

	tx := client.CreateAccount(keyPair.PublicKey, []byte{})

	tx.SetNonce(client.RandomNonce())
	tx.SetComputeLimit(100)

	signedTx := tx.SignPayer(defaultAccount)

	// client should return non-error if RPC call succeeds
	mockRPC.EXPECT().
		SendTransaction(ctx, gomock.Any()).
		Return(&observe.SendTransactionResponse{Hash: signedTx.Hash().Bytes()}, nil).
		Times(1)

	err := c.SendTransaction(ctx, *signedTx)
	Expect(err).ToNot(HaveOccurred())
}
