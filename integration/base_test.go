package integration_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"

	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/integration/network"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestContainer_Start(t *testing.T) {

	net := []*network.FlowNode{
		{
			Role:  flow.RoleCollection,
			Stake: 1000,
		},
		{
			Role:  flow.RoleConsensus,
			Stake: 1000,
		},
		{
			Role:  flow.RoleExecution,
			Stake: 1234,
		},
		{
			Role:  flow.RoleVerification,
			Stake: 4582,
		},
	}

	// Enable verbose logging
	testingdock.Verbose = true

	ctx := context.Background()

	flowNetwork, err := network.PrepareFlowNetwork(ctx, t, "mvp", net)
	require.NoError(t, err)

	flowNetwork.Suite.Start(ctx)
	defer flowNetwork.Suite.Close()

	var collectionNodeApiPort = ""
	for _, container := range flowNetwork.Containers {
		if container.Identity.Role == flow.RoleCollection {
			collectionNodeApiPort = container.Ports["api"]
		}
	}
	require.NotEqual(t, collectionNodeApiPort, "")

	sendTransaction(t, collectionNodeApiPort)

	// TODO Once we have observation API in place, query this API as the actual method of test assertion
	time.Sleep(15 * time.Second)
}

func sendTransaction(t *testing.T, apiPort string) {
	fmt.Printf("Sending tx to %s\n", apiPort)
	c, err := client.New("localhost:" + apiPort)
	require.NoError(t, err)

	// Generate key
	seed := make([]byte, 40)
	_, _ = rand.Read(seed)
	key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)
	require.NoError(t, err)

	tx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		PayerAccount:       sdk.RootAddress,
	}

	sig, err := keys.SignTransaction(tx, key)
	require.NoError(t, err)

	tx.AddSignature(sdk.RootAddress, sig)

	flowTxBody := flowTxBodyFromSDKTx(tx)
	fmt.Println("sending")
	err = c.SendTransaction(context.Background(), flowTxBody)
	fmt.Println("sending")
	require.NoError(t, err)
}

func flowTxBodyFromSDKTx(stx sdk.Transaction) flow.TransactionBody {

	scriptAccounts := make([]flow.Address, len(stx.ScriptAccounts))
	for i, ssa := range stx.ScriptAccounts {
		scriptAccounts[i] = flow.BytesToAddress(ssa.Bytes())
	}

	signs := make([]flow.AccountSignature, len(stx.Signatures))
	for i, sign := range stx.Signatures {
		signs[i] = flow.AccountSignature{
			Account:   flow.BytesToAddress(sign.Account.Bytes()),
			Signature: sign.Signature,
		}
	}

	txBody := flow.TransactionBody{
		ReferenceBlockID: flow.HashToID(stx.ReferenceBlockHash),
		Script:           stx.Script,
		PayerAccount:     flow.BytesToAddress(stx.PayerAccount.Bytes()),
		ScriptAccounts:   scriptAccounts,
		Signatures:       signs,
	}
	return txBody
}
