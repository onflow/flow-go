package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// timeout for individual actions
const defaultTimeout = time.Second * 10

func TestMVP_Network(t *testing.T) {
	colNode := testnet.NewNodeConfig(flow.RoleCollection)
	exeNode := testnet.NewNodeConfig(flow.RoleExecution)

	net := []testnet.NodeConfig{
		colNode,
		testnet.NewNodeConfig(flow.RoleCollection),
		exeNode,
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s")),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s")),
		testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithAdditionalFlag("--hotstuff-timeout=12s")),
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

	c, err := testnet.NewClientWithKey(":3569", key, flow.Emulator.Chain())
	require.NoError(t, err)

	//TODO commented out because main test requires genesis for sending tx
	// with valid reference block ID
	//runMVPTest(t, c)
	_ = c
}

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	root := net.Root()

	chain := root.Header.ChainID.Chain()

	client, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	// contract is not deployed, so script fails
	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	counter, err := readCounter(childCtx, client)
	cancel()
	require.Error(t, err)

	// deploy the contract
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = client.DeployContract(childCtx, root.ID(), CounterContract)
	cancel()
	require.NoError(t, err)

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, client)
		cancel()
		if err != nil {
			t.Log("EXECUTE SCRIPT ERR", err)
		}
		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	tx := unittest.TransactionBodyFixture(
		unittest.WithTransactionDSL(CreateCounterTx(chain)),
		unittest.WithReferenceBlock(root.ID()),
	)
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = client.SendTransaction(ctx, tx)
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
