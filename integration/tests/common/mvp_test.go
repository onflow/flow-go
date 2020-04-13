package common

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestMVP_Network(t *testing.T) {

	t.Skip()

	colNode := testnet.NewNodeConfig(flow.RoleCollection)
	exeNode := testnet.NewNodeConfig(flow.RoleExecution)

	net := []testnet.NodeConfig{
		colNode,
		exeNode,
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleVerification),
	}
	conf := testnet.NewNetworkConfig(net)

	// Enable verbose logging
	testingdock.Verbose = true

	ctx := context.Background()

	flowNetwork, err := testnet.PrepareFlowNetwork(t, "mvp", conf)
	require.NoError(t, err)

	flowNetwork.Start(ctx)
	defer flowNetwork.Remove()

	colContainer, ok := flowNetwork.ContainerByID(colNode.Identifier)
	require.True(t, ok)
	colNodeAPIPort := colContainer.Ports[testnet.ColNodeAPIPort]
	require.NotEqual(t, "", colNodeAPIPort)

	exeContainer, ok := flowNetwork.ContainerByID(exeNode.Identifier)
	require.True(t, ok)
	exeNodeAPIPort := exeContainer.Ports[testnet.ExeNodeAPIPort]
	require.NotEqual(t, "", exeNodeAPIPort)

	key, err := unittest.AccountKeyFixture()
	require.NoError(t, err)

	colClient, err := testnet.NewClientWithKey(fmt.Sprintf(":%s", colNodeAPIPort), key)
	require.NoError(t, err)

	exeClient, err := testnet.NewClientWithKey(fmt.Sprintf(":%s", exeNodeAPIPort), key)
	require.NoError(t, err)

	runMVPTest(t, colClient, exeClient)
}

func TestMVP_Emulator(t *testing.T) {

	//Start emulator manually for now, used for testing the test
	// TODO - start an emulator instance
	t.Skip()

	key, err := unittest.EmulatorRootKey()
	require.NoError(t, err)

	c, err := testnet.NewClientWithKey(":3569", key)
	require.NoError(t, err)

	runMVPTest(t, c, c)
}

func runMVPTest(t *testing.T, colClient *testnet.Client, exeClient *testnet.Client) {

	ctx := context.Background()

	// contract is not deployed, so script fails
	counter, err := readCounter(ctx, exeClient)
	require.Error(t, err)

	err = deployCounter(ctx, colClient)
	require.NoError(t, err)

	// script executes eventually, but no counter instance is created
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, exeClient)

		return err == nil && counter == -3
	}, 30*time.Second, time.Second)

	err = createCounter(ctx, colClient)
	require.NoError(t, err)

	// counter is created and incremented eventually
	require.Eventually(t, func() bool {
		counter, err = readCounter(ctx, exeClient)

		return err == nil && counter == 2
	}, 30*time.Second, time.Second)
}
