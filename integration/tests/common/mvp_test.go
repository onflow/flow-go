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

func TestMVP_Network(t *testing.T) {
	colNode := testnet.NewNodeConfig(flow.RoleCollection)
	exeNode := testnet.NewNodeConfig(flow.RoleExecution)

	net := []testnet.NodeConfig{
		colNode,
		testnet.NewNodeConfig(flow.RoleCollection),
		exeNode,
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess),
	}
	conf := testnet.NewNetworkConfig("mvp", net)

	ctx := context.Background()

	flowNetwork := testnet.PrepareFlowNetwork(t, conf)

	flowNetwork.Start(ctx)
	defer flowNetwork.Remove()

	accessPort := fmt.Sprintf(":%s", flowNetwork.AccessPorts[testnet.AccessNodeAPIPort])
	accessClient, err := testnet.NewClient(accessPort)
	require.NoError(t, err)

	runMVPTest(t, accessClient)
}

func TestMVP_Emulator(t *testing.T) {
	// Start emulator manually for now, used for testing the test
	// TODO - start an emulator instance
	t.Skip()

	key, err := unittest.EmulatorRootKey()
	require.NoError(t, err)

	c, err := testnet.NewClientWithKey(":3569", key)
	require.NoError(t, err)

	runMVPTest(t, c)
}

func runMVPTest(t *testing.T, accessClient *testnet.Client) {
	ctx := context.Background()

	// contract is not deployed, so script fails
	counter, err := readCounter(ctx, accessClient)
	require.Error(t, err)

	err = DeployCounter(ctx, accessClient)
	require.NoError(t, err)

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, accessClient)
		if err != nil {
			fmt.Println("EXECUTE SCRIPT ERR", err)
		}
		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	// TODO: Fix Cadence code
	err = createCounter(ctx, accessClient)
	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, accessClient)
		fmt.Printf("counter = %d\n", counter)
		fmt.Printf("error = %s\n", err)
		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
