package main

import (
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

func main() {
	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),

		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),

		testnet.NewNodeConfig(flow.RoleExecution),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess),
	}

	conf := testnet.NewNetworkConfig("mvp", net)

	_, err := testnet.BootstrapNetwork(conf, "./bootstrap")
	if err != nil {
		panic(err)
	}
}
