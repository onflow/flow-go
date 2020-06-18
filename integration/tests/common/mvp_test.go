package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/encoding/json"
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

	c, err := testnet.NewClientWithKey(":3569", key, flow.Emulator.Chain())
	require.NoError(t, err)

	//TODO commented out because main test requires genesis for sending tx
	// with valid reference block ID
	//runMVPTest(t, c)
	_ = c
}

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	genesis := net.Genesis()

	chain := genesis.Header.ChainID.Chain()

	serviceAccountClient, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	//create new account to deploy Counter to
	//account, key, signer := GetAccount(chain)

	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	err = createAccount(childCtx, serviceAccountClient, genesis)
	cancel()

	var newAccountAddress flow.Address = flow.EmptyAddress

	require.Eventually(t, func() bool {
		childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
		responses, err := serviceAccountClient.Events(childCtx, "flow.AccountCreated")
		cancel()

		if err != nil {
			return false
		}

		found := false

		for _, response := range responses {
			for _, e := range response.Events {
				if e.Type == string(flow.EventAccountCreated) { //just to be sure

					event, err := json.Decode(e.Payload)
					if err != nil {
						fmt.Printf("decoding payload err = %s\n", err)
					}

					newAccountAddress = event.ToGoValue().([]interface{})[0].(flow.Address)

					found = true
				}
			}
		}

		return found
	}, 30*time.Second, time.Second)

	require.NoError(t, err)

	// contract is not deployed, so script fails
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	counter, err := readCounter(childCtx, serviceAccountClient)
	cancel()
	require.Error(t, err)

	fmt.Printf("service address1: %s\n", chain.ServiceAddress().Short())

	// deploy the contract
	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.DeployContract(childCtx, genesis.ID(), CounterContract)
	cancel()
	require.NoError(t, err)

	fmt.Printf("service address: %s\n", chain.ServiceAddress().Short())

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)

		fmt.Printf("service address script: %s\n", chain.ServiceAddress().Short())

		counter, err = readCounter(ctx, serviceAccountClient)
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
		SetProposalKey(chain.ServiceAddress(), 0, 0).
		SetPayer(chain.ServiceAddress()).
		AddAuthorizer(chain.ServiceAddress())

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = serviceAccountClient.SendTransaction(ctx, *tx)
	cancel()

	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, serviceAccountClient)
		cancel()

		t.Logf("read counter: counter=%d, err=%s", counter, err)
		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
