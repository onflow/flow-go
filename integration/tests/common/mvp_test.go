package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	//sdk "github.com/onflow/flow-go-sdk"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	//"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func TestMVP_Network(t *testing.T) {
	colNode := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.WarnLevel))
	exeNode := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel))

	net := []testnet.NodeConfig{
		colNode,
		testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.WarnLevel)),
		exeNode,
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s"), testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s"), testnet.WithLogLevel(zerolog.WarnLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s"), testnet.WithLogLevel(zerolog.WarnLevel)),
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

	key, err := unittest.EmulatorRootKey()
	require.NoError(t, err)

	c, err := testnet.NewClientWithKey(":3569", key)
	require.NoError(t, err)

	//TODO commented out because main test requires genesis for sending tx
	// with valid reference block ID
	//runMVPTest(t, c)
	_ = c
}

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	client, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(t, err)

	genesis := net.Genesis()

	// contract is not deployed, so script fails
	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	counter, err := readCounter(childCtx, client)
	cancel()
	require.Error(t, err)

	fmt.Printf("service address1: %s\n", flow.ServiceAddress().Short())

	// deploy the contract
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = client.DeployContract(childCtx, genesis.ID(), CounterContract)
	cancel()
	require.NoError(t, err)

	fmt.Printf("service address: %s\n", flow.ServiceAddress().Short())

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)

		fmt.Printf("service address script: %s\n", flow.ServiceAddress().Short())

		counter, err = readCounter(ctx, client)
		cancel()
		if err != nil {
			t.Log("EXECUTE SCRIPT ERR", err)
		}
		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	//tx := unittest.TransactionBodyFixture(
	//	unittest.WithTransactionDSL(CreateCounterTx),
	//	unittest.WithReferenceBlock(genesis.ID()),
	//)

	//sdk.ServiceAddress(sdk.Mainnet)

	tx := flow.NewTransactionBody().
		SetScript(unittest.NoopTxScript()).
		SetReferenceBlockID(genesis.ID()).
		SetProposalKey(flow.ServiceAddress(), 0, 0).
		SetPayer(flow.ServiceAddress()).
		AddAuthorizer(flow.ServiceAddress())

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = client.SendTransaction(ctx, *tx)
	cancel()

	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, client)
		cancel()

		t.Logf("read counter: counter=%d, err=%s", counter, err)
		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
