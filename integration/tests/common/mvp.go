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
	"github.com/onflow/flow-go/utils/unittest"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func MVP_Network(t *testing.T) {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", t.Name()).
		Logger()
	logger.Info().Msgf("================> START TESTING")
	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)
	defer func() {
		logger.Info().Msgf("================> Start TearDownTest")
		flowNetwork.Remove()
		logger.Info().Msgf("================> Finish TearDownTest")
	}()

	runMVPTest(t, ctx, flowNetwork)
}

func MVP_Bootstrap(t *testing.T) {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "suite.go").
		Str("testcase", t.Name()).
		Logger()
	logger.Info().Msgf("================> START TESTING")
	unittest.SkipUnless(t, unittest.TEST_WIP, "skipping to be re-visited in https://github.com/dapperlabs/flow-go/issues/5451")

	testingdock.Verbose = false

	flowNetwork := testnet.PrepareFlowNetwork(t, buildMVPNetConfig())
	defer func() {
		logger.Info().Msgf("================> Start TearDownTest")
		flowNetwork.Remove()
		logger.Info().Msgf("================> Finish TearDownTest")
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork.Start(ctx)

	initialRoot := flowNetwork.Root()
	chain := initialRoot.Header.ChainID.Chain()

	client, err := testnet.NewClient(fmt.Sprintf(":%s", flowNetwork.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	t.Log("@@ running mvp test 1")

	// run mvp test to build a few blocks
	runMVPTest(t, ctx, flowNetwork)

	t.Log("@@ finished running mvp test 1")

	// download root snapshot from access node
	snapshot, err := client.GetLatestProtocolSnapshot(ctx)
	require.NoError(t, err)

	// verify that the downloaded snapshot is not for the root block
	header, err := snapshot.Head()
	require.NoError(t, err)
	assert.True(t, header.ID() != initialRoot.Header.ID())

	t.Log("@@ restarting network with new root snapshot")

	flowNetwork.StopContainers()
	flowNetwork.RemoveContainers()

	// pick 1 consensus node to restart with empty database and downloaded snapshot
	con1 := flowNetwork.Identities().
		Filter(filter.HasRole(flow.RoleConsensus)).
		Sample(1)[0]

	t.Log("@@ booting from non-root state on consensus node ", con1.NodeID)

	flowNetwork.DropDBs(filter.HasNodeID(con1.NodeID))
	con1Container := flowNetwork.ContainerByID(con1.NodeID)
	con1Container.DropDB()
	con1Container.WriteRootSnapshot(snapshot)

	t.Log("@@ running mvp test 2")

	flowNetwork.Start(ctx)

	// Run MVP tests
	runMVPTest(t, ctx, flowNetwork)

	t.Log("@@ finished running mvp test 2")
}

func buildMVPNetConfig() testnet.NetworkConfig {
	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	consensusConfigs := []func(config *testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.FatalLevel),
	}

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithAdditionalFlag("--extensive-logging=true")),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel)),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
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

	t.Log(">> new account address: ", newAccountAddress)

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

	t.Log(">> funding new account...")

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
	counter, err := ReadCounter(childCtx, accountClient, newAccountAddress)
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

	t.Log(">> creating counter...")

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = accountClient.SignAndSendTransaction(ctx, createCounterTx)
	cancel()

	require.NoError(t, err)

	resp, err := accountClient.WaitForSealed(context.Background(), createCounterTx.ID())
	require.NoError(t, err)

	require.NoError(t, resp.Error)
	t.Log(resp)

	t.Log(">> awaiting counter incrementing...")

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = ReadCounter(ctx, serviceAccountClient, newAccountAddress)
		cancel()

		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
