package staging

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

const testnetAddr = "35.247.77.6:9000"

func TestMVP_Network(t *testing.T) {

	rootKey, err := crypto.DecodePrivateKey(crypto.ECDSA_P256, []byte("f87db879307702010104208ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43a00a06082a8648ce3d030107a14403420004b60899344f1779bb4c6df5a81db73e5781d895df06dee951e813eba99e76dd3c9288cceb5a5a9ada390671f60b71f3fd2653ca5c4e1ccc6f8b5a62be6d17256a0201"))
	require.NoError(t, err)

	privateKey := &flow.AccountPrivateKey{
		PrivateKey: rootKey,
		SignAlgo:   crypto.ECDSA_P256,
		HashAlgo:   hash.SHA2_256,
	}

	accessClient, err := testnet.NewClientWithKey(testnetAddr, privateKey)
	require.NoError(t, err)

	ctx := context.Background()

	// contract is not deployed, so script fails
	counter, err := readCounter(ctx, accessClient)
	require.Error(t, err)

	err = deployCounter(ctx, accessClient)
	require.NoError(t, err)

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, accessClient)

		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	// err = common.createCounter(ctx, accessClient)
	// require.NoError(t, err)

	// // counter is created and incremented eventually
	// require.Eventually(t, func() bool {
	// 	counter, err = common.readCounter(ctx, accessClient)

	// 	return err == nil && counter == 2
	// }, 30*time.Second, time.Second)
}

func Test_Network_GetAccount(t *testing.T) {
	var w accountPrivateKeyWrapper

	b, err := hex.DecodeString("f87db879307702010104208ae3d0461cfed6d6f49bfc25fa899351c39d1bd21fdba8c87595b6c49bb4cc43a00a06082a8648ce3d030107a14403420004b60899344f1779bb4c6df5a81db73e5781d895df06dee951e813eba99e76dd3c9288cceb5a5a9ada390671f60b71f3fd2653ca5c4e1ccc6f8b5a62be6d17256a0201")
	require.NoError(t, err)

	err = rlp.NewEncoder().Decode(b, &w)
	require.NoError(t, err)

	rootKey, err := crypto.DecodePrivateKey(crypto.ECDSA_P256, w.EncodedPrivateKey)
	require.NoError(t, err)

	privateKey := &flow.AccountPrivateKey{
		PrivateKey: rootKey,
		SignAlgo:   crypto.ECDSA_P256,
		HashAlgo:   hash.SHA2_256,
	}

	accessClient, err := testnet.NewClientWithKey(testnetAddr, privateKey)
	require.NoError(t, err)

	ctx := context.Background()

	acc, err := accessClient.GetAccount(ctx, flow.HexToAddress("01"))
	require.NoError(t, err)

	fmt.Println(acc)

	t.Fail()
}

type accountPrivateKeyWrapper struct {
	EncodedPrivateKey []byte
	SignAlgo          uint
	HashAlgo          uint
}
