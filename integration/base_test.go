package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"testing"
	"time"

	"github.com/m4ksio/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/cadence/encoding"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/integration/dsl"
	. "github.com/dapperlabs/flow-go/integration/network"
	"github.com/dapperlabs/flow-go/integration/testclient"
	"github.com/dapperlabs/flow-go/model/flow"
)

func Test_MVPNetwork(t *testing.T) {

	t.Skip()

	net := []*FlowNode{
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

	flowNetwork, err := PrepareFlowNetwork(ctx, t, "mvp", net)
	require.NoError(t, err)

	flowNetwork.Suite.Start(ctx)
	defer flowNetwork.Suite.Close()

	var collectionNodeApiPort = ""
	for _, container := range flowNetwork.Containers {
		if container.Identity.Role == flow.RoleCollection {
			collectionNodeApiPort = container.Ports["api"]
		}
	}

	var executionNodeApiPort = ""
	for _, container := range flowNetwork.Containers {
		if container.Identity.Role == flow.RoleExecution {
			executionNodeApiPort = container.Ports["api"]
		}
	}

	require.NotEqual(t, collectionNodeApiPort, "")
	require.NotEqual(t, executionNodeApiPort, "")

	key, err := generateRandomKey()
	require.NoError(t, err)

	collectionClient, err := testclient.New(fmt.Sprintf(":%s", collectionNodeApiPort), key)
	require.NoError(t, err)

	executionClient, err := testclient.New(fmt.Sprintf(":%s", executionNodeApiPort), key)
	require.NoError(t, err)

	runMVPTest(t, collectionClient, executionClient)
}

func Test_MVPEmulator(t *testing.T) {

	//Start emulator manually for now, used for testing the test
	// TODO - start an emulator instance
	t.Skip()

	key, err := getEmulatorKey()
	require.NoError(t, err)

	c, err := testclient.New(":3569", key)
	require.NoError(t, err)

	runMVPTest(t, c, c)
}

func runMVPTest(t *testing.T, collectionClient *testclient.TestClient, executionClient *testclient.TestClient) {

	ctx := context.Background()

	// contract is not deployed, so script fails
	counter, err := readCounter(ctx, executionClient)
	require.Error(t, err)

	err = deployCounter(ctx, collectionClient)
	require.NoError(t, err)

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, executionClient)

		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	err = createCounter(ctx, collectionClient)
	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, executionClient)

		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}

func deployCounter(ctx context.Context, client *testclient.TestClient) error {

	contract := dsl.Contract{
		Name: "Testing",
		Members: []dsl.CadenceCode{
			dsl.Resource{
				Name: "Counter",
				Code: `
				pub var count: Int

				init() {
					self.count = 0
				}
				pub fun add(_ count: Int) {
					self.count = self.count + count
				}`,
			},
			dsl.Code(`
				pub fun createCounter(): @Counter {
					return <-create Counter()
      			}`,
			),
		},
	}

	return client.DeployContract(ctx, contract)
}

func readCounter(ctx context.Context, client *testclient.TestClient) (int, error) {

	script := dsl.Main{
		ReturnType: "Int",
		Code:       "return getAccount(0x01).published[&Testing.Counter]?.count ?? -3",
	}

	res, err := client.ExecuteScript(ctx, script)
	if err != nil {
		return 0, err
	}

	decoder := encoding.NewDecoder(bytes.NewReader(res))
	i, err := decoder.DecodeInt()
	if err != nil {
		return 0, err
	}

	return int(i.Value.Int64()), nil
}

func createCounter(ctx context.Context, client *testclient.TestClient) error {
	rootAddress := flow.BytesToAddress(big.NewInt(1).Bytes())
	return client.SendTransaction(ctx, dsl.Transaction{
		Import: dsl.Import{Address: rootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				if signer.storage[Testing.Counter] == nil {
				let existing <- signer.storage[Testing.Counter] <- Testing.createCounter()
            	    destroy existing
            	    signer.published[&Testing.Counter] = &signer.storage[Testing.Counter] as Testing.Counter
            	}
            	signer.published[&Testing.Counter]?.add(2)`),
		}})

}

func generateRandomKey() (*flow.AccountPrivateKey, error) {
	seed := make([]byte, 40)
	_, _ = rand.Read(seed)
	key, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, seed)
	if err != nil {
		return nil, err
	}

	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   crypto.SHA3_256,
	}, nil

}

func getEmulatorKey() (*flow.AccountPrivateKey, error) {
	key, err := crypto.DecodePrivateKey(crypto.ECDSA_P256, []byte("f87db87930770201010420ae2cc975dcbdd0ebc56f268b1d8a95834c2955970aea27042d35ec9f298b9e5aa00a06082a8648ce3d030107a1440342000417f5a527137785d2d773fee84b4c7ee40266a1dd1f36ddd46ecf25db6df6a499459629174de83256f2a44ebd4325b9def67d523b755a8926218c4efb7904f8ce0203"))
	if err != nil {
		return nil, err
	}
	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   crypto.SHA3_256,
	}, nil
}
