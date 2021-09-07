package testnet_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

// var defaultRegistry = "gcr.io/dl-flow"
var defaultRegistry = "gcr.io/flow-container-registry"

// data for easy asserting interesting fields
type nodeInfo struct {
	image   string
	name    string
	address string
}

func TestNetworkSetupBasic(t *testing.T) {
	registry := os.Getenv("CONTAINER_REGISTRY")
	if len(registry) > 0 {
		defaultRegistry = registry
	}

	nodes := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleExecution),
		testnet.NewNodeConfig(flow.RoleVerification),
	}
	conf := testnet.NewNetworkConfig("meta_test_basic", nodes)

	flowNetwork := testnet.PrepareFlowNetwork(t, conf)
	defer flowNetwork.Cleanup()

	assert.Len(t, flowNetwork.Containers, len(nodes))

	realData := getNodeInfos(flowNetwork)

	expectedData := []nodeInfo{
		{image: defaultRegistry + "/collection:latest", name: "collection_1", address: "collection_1:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_1", address: "consensus_1:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_2", address: "consensus_2:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_3", address: "consensus_3:2137"},
		{image: defaultRegistry + "/execution:latest", name: "execution_1", address: "execution_1:2137"},
		{image: defaultRegistry + "/verification:latest", name: "verification_1", address: "verification_1:2137"},
	}

	assert.Subset(t, realData, expectedData)
}

func TestNetworkSetupMultipleNodes(t *testing.T) {

	nodes := []testnet.NodeConfig{
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleCollection),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleConsensus),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleVerification),
		testnet.NewNodeConfig(flow.RoleExecution),
	}
	conf := testnet.NewNetworkConfig("meta_test_multinodes", nodes)

	flowNetwork := testnet.PrepareFlowNetwork(t, conf)
	defer flowNetwork.Cleanup()

	assert.Len(t, flowNetwork.Containers, len(nodes))

	realData := getNodeInfos(flowNetwork)

	expectedData := []nodeInfo{
		{image: defaultRegistry + "/collection:latest", name: "collection_1", address: "collection_1:2137"},
		{image: defaultRegistry + "/collection:latest", name: "collection_2", address: "collection_2:2137"},
		{image: defaultRegistry + "/collection:latest", name: "collection_3", address: "collection_3:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_1", address: "consensus_1:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_2", address: "consensus_2:2137"},
		{image: defaultRegistry + "/consensus:latest", name: "consensus_3", address: "consensus_3:2137"},
		{image: defaultRegistry + "/verification:latest", name: "verification_1", address: "verification_1:2137"},
		{image: defaultRegistry + "/verification:latest", name: "verification_2", address: "verification_2:2137"},
		{image: defaultRegistry + "/verification:latest", name: "verification_3", address: "verification_3:2137"},
		{image: defaultRegistry + "/execution:latest", name: "execution_1", address: "execution_1:2137"},
	}

	assert.Subset(t, realData, expectedData)
}

func getNodeInfos(flowNetwork *testnet.FlowNetwork) []nodeInfo {
	realData := make([]nodeInfo, 0, len(flowNetwork.Containers))

	for _, container := range flowNetwork.Containers {
		realData = append(realData, nodeInfo{
			image:   container.Image,
			name:    container.Name(),
			address: container.Config.Address,
		})
	}
	return realData
}
