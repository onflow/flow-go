package integration_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/network"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestContainer_Start(t *testing.T) {

	t.Skip()

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

func Test_emulator(t *testing.T) {
	sendTransaction(t, "3569")
}

func sendTransaction(t *testing.T, apiPort string) {
	fmt.Printf("Sending tx to %s\n", apiPort)
	c, err := client.New("localhost:" + apiPort)
	require.NoError(t, err)

	// Generate key
	//seed := make([]byte, 40)
	//_, _ = rand.Read(seed)
	//key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)

	prKeyBytes, err := hex.DecodeString("f87db87930770201010420f851e880fae1abe194f6ec8b876c0833b41d0ebbbaa24d26696bf32a0025a7b7a00a06082a8648ce3d030107a1440342000480954a16bbffabe1b34fb5194bff0a0bea07a37d1454ec37bd216e463f9915b84c3c455d36afc42507f2a1e0998ac5965376ef5a532fcc53a0a66f1a99c2242d0203")
	require.NoError(t, err)
	keySdk, err := sdk.DecodeAccountPrivateKey(prKeyBytes)

	require.NoError(t, err)

	nonce := 2137

	script := `
	pub contract Counter {
		pub resource Counter {
		  pub var counter: Int
		
		  init() {
			  self.counter = 3
		  }
		
		  pub fun bump() {
			  self.counter = self.counter + 1
		  }
		}
		
		pub fun new(): @Counter {
		  return <-create Counter()
		}
  	}
	
	transaction {

		prepare(signer: Account) {
			var counter:@Counter.Counter? <- Counter.new()
			signer.storage[Counter.Counter] <-> counter
			destroy counter

			signer.published[&Counter.Counter] = &signer.storage[Counter.Counter] as Counter.Counter
		}
	}

	`

	tx := sdk.Transaction{
		Script:             []byte(script),
		ReferenceBlockHash: nil,
		Nonce:              uint64(len(script)+nonce+4),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
		ScriptAccounts: 	[]sdk.Address{sdk.RootAddress},
	}


	//tx.ScriptAccounts = append(tx.ScriptAccounts, sdk.RootAddress)


	sig, err := keys.SignTransaction(tx, keySdk)
	require.NoError(t, err)



	tx.AddSignature(sdk.RootAddress, sig)


	err = c.SendTransaction(context.Background(), tx)
	require.NoError(t, err)
}
