package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func TestMVP_Network(t *testing.T) {
	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)
	defer flowNetwork.Remove()

	runMVPTest(t, ctx, flowNetwork)
}

func TestMVP_Bootstrap(t *testing.T) {

	// skipping to be re-visited in https://github.com/dapperlabs/flow-go/issues/5451
	t.Skip()

	testingdock.Verbose = false

	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig())
	defer flowNetwork.Remove()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)

	initialRoot := flowNetwork.Root()
	chain := initialRoot.Header.ChainID.Chain()

	client, err := testnet.NewClient(fmt.Sprintf(":%s", flowNetwork.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	fmt.Println("@@ running mvp test 1")

	// run mvp test to build a few blocks
	runMVPTest(t, ctx, flowNetwork)

	fmt.Println("@@ finished running mvp test 1")

	// download root snapshot from access node
	snapshot, err := client.GetLatestProtocolSnapshot(ctx)
	require.NoError(t, err)

	// verify that the downloaded snapshot is not for the root block
	header, err := snapshot.Head()
	require.NoError(t, err)
	assert.True(t, header.ID() != initialRoot.Header.ID())

	fmt.Println("@@ restarting network with new root snapshot")

	flowNetwork.StopContainers()
	flowNetwork.RemoveContainers()

	// pick 1 consensus node to restart with empty database and downloaded snapshot
	con1 := flowNetwork.Identities().
		Filter(filter.HasRole(flow.RoleConsensus)).
		Sample(1)[0]

	fmt.Println("@@ booting from non-root state on consensus node ", con1.NodeID)

	flowNetwork.DropDBs(filter.HasNodeID(con1.NodeID))
	con1Container := flowNetwork.ContainerByID(con1.NodeID)
	con1Container.DropDB()
	con1Container.WriteRootSnapshot(snapshot)

	fmt.Println("@@ running mvp test 2")

	flowNetwork.Start(ctx)

	// Run MVP tests
	runMVPTest(t, ctx, flowNetwork)

	fmt.Println("@@ finished running mvp test 2")
}

func TestMVP_Emulator(t *testing.T) {
	// Start emulator manually for now, used for testing the test
	// TODO - start an emulator instance
	t.Skip()

	// key, err := unittest.EmulatorRootKey()
	// require.NoError(t, err)

	// c, err := testnet.NewClientWithKey(":3569", key, flow.Emulator.Chain())
	// require.NoError(t, err)

	//TODO commented out because main test requires root for sending tx
	// with valid reference block ID
	//runMVPTest(t, c)
	// _ = c
}

func buildMVPNetConfig() testnet.NetworkConfig {
	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.InfoLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.InfoLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithDebugImage(false)),
		testnet.NewNodeConfig(flow.RoleAccess),
		testnet.NewNodeConfig(flow.RoleAccess),
	}

	return testnet.NewNetworkConfig("mvp", net)
}

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	chain := net.Root().Header.ChainID.Chain()

	serviceAccountClient, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	latestBlockID, err := serviceAccountClient.GetLatestBlockID(ctx)
	require.NoError(t, err)

	// create new account to deploy Counter to
	accountPrivateKey := RandomPrivateKey()

	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(serviceAccountClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx := templates.CreateAccount(
		[]*sdk.AccountKey{accountKey},
		[]templates.Contract{
			{
				Name:   CounterContract.Name,
				Source: CounterContract.ToCadence(),
			},
		},
		serviceAddress).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetGasLimit(9999)

	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SignAndSendTransaction(ctx, createAccountTx)
	require.NoError(t, err)

	cancel()

	// wait for account to be created
	accountCreationTxRes, err := serviceAccountClient.WaitForSealed(context.Background(), createAccountTx.ID())
	require.NoError(t, err)
	t.Log(accountCreationTxRes)

	var newAccountAddress sdk.Address
	for _, event := range accountCreationTxRes.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
		}
	}
	require.NotEqual(t, sdk.EmptyAddress, newAccountAddress)

	fmt.Println(">> new account address: ", newAccountAddress)

	// Generate the fund account transaction (so account can be used as a payer)
	fundAccountTx := sdk.NewTransaction().
		SetScript([]byte(fmt.Sprintf(`
			import FungibleToken from 0x%s
			import FlowToken from 0x%s

			transaction(amount: UFix64, recipient: Address) {
			  let sentVault: @FungibleToken.Vault
			  prepare(signer: AuthAccount) {
				let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
				  ?? panic("failed to borrow reference to sender vault")
				self.sentVault <- vaultRef.withdraw(amount: amount)
			  }
			  execute {
				let receiverRef =  getAccount(recipient)
				  .getCapability(/public/flowTokenReceiver)
				  .borrow<&{FungibleToken.Receiver}>()
					?? panic("failed to borrow reference to recipient vault")
				receiverRef.deposit(from: <-self.sentVault)
			  }
			}`,
			fvm.FungibleTokenAddress(chain).Hex(),
			fvm.FlowTokenAddress(chain).Hex()))).
		AddAuthorizer(serviceAddress).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress).
		SetGasLimit(9999)

	err = fundAccountTx.AddArgument(cadence.UFix64(1_0000_0000))
	require.NoError(t, err)
	err = fundAccountTx.AddArgument(cadence.NewAddress(newAccountAddress))
	require.NoError(t, err)

	fmt.Println(">> funding new account...")

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SignAndSendTransaction(ctx, fundAccountTx)
	require.NoError(t, err)

	cancel()

	fundCreationTxRes, err := serviceAccountClient.WaitForSealed(context.Background(), fundAccountTx.ID())
	require.NoError(t, err)
	t.Log(fundCreationTxRes)

	accountClient, err := testnet.NewClientWithKey(
		fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]),
		newAccountAddress,
		accountPrivateKey,
		chain,
	)
	require.NoError(t, err)

	// contract is deployed, but no instance is created yet
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	counter, err := readCounter(childCtx, accountClient, newAccountAddress)
	cancel()
	require.NoError(t, err)
	require.Equal(t, -3, counter)

	// create counter instance
	createCounterTx := sdk.NewTransaction().
		SetScript([]byte(CreateCounterTx(newAccountAddress).ToCadence())).
		SetReferenceBlockID(sdk.Identifier(latestBlockID)).
		SetProposalKey(newAccountAddress, 0, 0).
		SetPayer(newAccountAddress).
		AddAuthorizer(newAccountAddress).
		SetGasLimit(9999)

	fmt.Println(">> creating counter...")

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = accountClient.SignAndSendTransaction(ctx, createCounterTx)
	cancel()

	require.NoError(t, err)

	resp, err := accountClient.WaitForSealed(context.Background(), createCounterTx.ID())
	require.NoError(t, err)

	require.NoError(t, resp.Error)
	t.Log(resp)

	fmt.Println(">> awaiting counter incrementing...")

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, serviceAccountClient, newAccountAddress)
		cancel()

		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
