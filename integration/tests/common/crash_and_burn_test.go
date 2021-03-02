package common

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

func TestCrashAndBurnBehaviour(t *testing.T) {
	collectionConfigs := []func(*testnet.NodeConfig){
		testnet.WithAdditionalFlag("--hotstuff-timeout=12s"),
		testnet.WithAdditionalFlag("--block-rate-delay=100ms"),
		testnet.WithLogLevel(zerolog.WarnLevel),
	}

	consensusConfigs := append(collectionConfigs,
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-verification-seal-approvals=%d", 1)),
		testnet.WithAdditionalFlag(fmt.Sprintf("--required-construction-seal-approvals=%d", 1)),
		testnet.WithLogLevel(zerolog.DebugLevel),
	)

	net := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleCollection, collectionConfigs...),
		testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.DebugLevel)),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleConsensus, consensusConfigs...),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithDiskSpace(100)),
	}

	conf := testnet.NewNetworkConfig("mvp", net)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flowNetwork := testnet.PrepareFlowNetwork(t, conf)

	flowNetwork.Start(ctx)
	defer flowNetwork.Remove()

	runMVPTest(t, ctx, flowNetwork)
}
