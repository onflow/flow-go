package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
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

	//TODO commented out because main test requires root for sending tx
	// with valid reference block ID
	//runMVPTest(t, c)
	_ = c
}

func runMVPTest(t *testing.T, ctx context.Context, net *testnet.FlowNetwork) {

	root := net.Root()
	chain := root.Header.ChainID.Chain()

	serviceAccountClient, err := testnet.NewClient(fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]), chain)
	require.NoError(t, err)

	//create new account to deploy Counter to
	accountPrivateKey, err := testutil.GenerateAccountPrivateKey()
	require.NoError(t, err)

	childCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	err = createAccount(childCtx, serviceAccountClient, root, []byte(CounterContract.ToCadence()), accountPrivateKey.PublicKey(fvm.AccountKeyWeightThreshold))
	cancel()

	var newAccountAddress = flow.EmptyAddress

	// wait for account to be created
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

					newAccountAddress = event.ToGoValue().([]interface{})[0].([flow.AddressLength]byte)

					found = true
				}
			}
		}

		return found
	}, 30*time.Second, time.Second)

	accountClient, err := testnet.NewClientWithKey(
		fmt.Sprintf(":%s", net.AccessPorts[testnet.AccessNodeAPIPort]),
		&accountPrivateKey,
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
	tx := flow.NewTransactionBody().
		SetScript([]byte(CreateCounterTx(newAccountAddress).ToCadence())).
		SetReferenceBlockID(root.ID()).
		SetProposalKey(newAccountAddress, 0, 0).
		SetPayer(newAccountAddress).
		AddAuthorizer(newAccountAddress)

	childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
	err = accountClient.SignAndSendTransaction(ctx, *tx)
	cancel()

	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		childCtx, cancel = context.WithTimeout(ctx, defaultTimeout)
		counter, err = readCounter(ctx, serviceAccountClient, newAccountAddress)
		cancel()

		t.Logf("read counter: counter=%d, err=%s", counter, err)
		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
