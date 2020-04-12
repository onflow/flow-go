package tests

import (
	"context"
	"testing"
	"time"

	"github.com/dapperlabs/testingdock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestAccess(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000001")
		})
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, func(c *testnet.NodeConfig) {
			c.Identifier, _ = flow.HexStringToIdentifier("0000000000000000000000000000000000000000000000000000000000000002")
		})
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus)
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus)
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus)
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution)
		verNode  = testnet.NewNodeConfig(flow.RoleVerification)
		accNode  = testnet.NewNodeConfig(flow.RoleAccess)
	)

	nodes := []testnet.NodeConfig{colNode1, colNode2, conNode1, conNode2, conNode3, exeNode, verNode, accNode}
	conf := testnet.NetworkConfig{Nodes: nodes}

	testingdock.Verbose = true

	ctx := context.Background()

	net, err := testnet.PrepareFlowNetwork(t, "access", conf)
	require.Nil(t, err)

	net.Start(ctx)
	defer net.Cleanup()

	time.Sleep(10 * time.Second)
	err = net.StopContainers()

}
