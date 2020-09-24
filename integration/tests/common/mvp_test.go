package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func TestMVP_Network(t *testing.T) {
	consensusConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	colNode := testnet.NewNodeConfig(flow.RoleCollection, consensusConfigs...)
	exeNode := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel))

	net := []testnet.NodeConfig{
		colNode,
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.WarnLevel)),
		exeNode,
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess),
	}
	conf := testnet.NewNetworkConfig("mvp", net)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork := testnet.PrepareFlowNetwork(t, conf)

	flowNetwork.Start(ctx)
	defer flowNetwork.Remove()

	runMVPTest(t, ctx, flowNetwork)
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

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	root := net.Root()
	chain := root.Header.ChainID.Chain()

	serviceAccountClient, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	//create new account to deploy Counter to
	accountPrivateKey := RandomPrivateKey()

	require.NoError(t, err)
	accountKey := sdk.NewAccountKey().
		FromPrivateKey(accountPrivateKey).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	serviceAddress := sdk.Address(serviceAccountClient.Chain.ServiceAddress())

	// Generate the account creation transaction
	createAccountTx := templates.CreateAccount([]*sdk.AccountKey{accountKey}, []byte(CounterContract.ToCadence()), serviceAddress).
		SetReferenceBlockID(sdk.Identifier(root.ID())).
		SetProposalKey(serviceAddress, 0, serviceAccountClient.GetSeqNumber()).
		SetPayer(serviceAddress)

	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SignAndSendTransaction(ctx, createAccountTx)
	require.NoError(t, err)

	cancel()

	// wait for account to be created
	accountCreationTxRes, err := serviceAccountClient.WaitForSealed(context.Background(), createAccountTx.ID())
	require.NoError(t, err)

	var newAccountAddress sdk.Address
	for _, event := range accountCreationTxRes.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			newAccountAddress = accountCreatedEvent.Address()
		}
	}

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
		SetReferenceBlockID(sdk.Identifier(root.ID())).
		SetProposalKey(newAccountAddress, 0, 0).
		SetPayer(newAccountAddress).
		AddAuthorizer(newAccountAddress)

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = accountClient.SignAndSendTransaction(ctx, createCounterTx)
	cancel()

	require.NoError(t, err)

	resp, err := accountClient.WaitForSealed(context.Background(), createCounterTx.ID())
	require.NoError(t, err)

	require.NoError(t, resp.Error)
	t.Log(resp)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, serviceAccountClient, newAccountAddress)
		cancel()

		t.Logf("read counter: counter=%d, err=%v", counter, err)
		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
