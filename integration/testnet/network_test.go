package testnet_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/integration/testnet"
)

// data for easy asserting interesting fields
type nodeInfo struct {
	image   string
	name    string
	address string
}

func TestNetworkSetupBasic(t *testing.T) {

	net := []*testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleExecution),
		testnet.NewNodeConfig(flow.RoleVerification),
	}

	flowNetwork, err := testnet.PrepareFlowNetwork(context.Background(), t, "testing", net)
	require.NoError(t, err)

	assert.Len(t, flowNetwork.Containers, len(net))

	realData := getNodeInfos(flowNetwork)

	expectedData := []nodeInfo{
		{image: "gcr.io/dl-flow/collection:latest", name: "collection", address: "collection:2137"},
		{image: "gcr.io/dl-flow/consensus:latest", name: "consensus", address: "consensus:2137"},
		{image: "gcr.io/dl-flow/execution:latest", name: "execution", address: "execution:2137"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification", address: "verification:2137"},
	}

	assert.Subset(t, realData, expectedData)
}

func TestNetworkSetupMultipleNodes(t *testing.T) {

	net := []*testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleExecution),
	}

	flowNetwork, err := testnet.PrepareFlowNetwork(context.Background(), t, "testing", net)
	require.NoError(t, err)

	assert.Len(t, flowNetwork.Containers, len(net))

	realData := getNodeInfos(flowNetwork)

	expectedData := []nodeInfo{
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_0", address: "collection_0:2137"},
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_1", address: "collection_1:2137"},
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_2", address: "collection_2:2137"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_0", address: "verification_0:2137"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_1", address: "verification_1:2137"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_2", address: "verification_2:2137"},
		{image: "gcr.io/dl-flow/execution:latest", name: "execution", address: "execution:2137"},
	}

	assert.Subset(t, realData, expectedData)
}

func getNodeInfos(flowNetwork *testnet.FlowNetwork) []nodeInfo {
	realData := make([]nodeInfo, len(flowNetwork.Containers))

	for i, container := range flowNetwork.Containers {
		realData[i] = nodeInfo{
			image:   container.Image,
			name:    container.Name,
			address: container.Identity.Address,
		}
	}
	return realData
}
