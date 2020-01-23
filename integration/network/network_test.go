package network_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"

	"github.com/dapperlabs/flow-go/integration/network"
)

// data for easy asserting interesting fields
type data struct {
	image   string
	name    string
	address string
}

func TestNetworkSetupBasic(t *testing.T) {

	net := []*network.FlowNode{
		{
			Role:  flow.RoleCollection,
			Stake: 1000,
		},
		{
			Role:  flow.RoleConsensus,
			Stake: 1000,
		},
		{
			Role:  flow.RoleExecution,
			Stake: 1234,
		},
		{
			Role:  flow.RoleVerification,
			Stake: 4582,
		},
	}

	flowNetwork, err := network.PrepareFlowNetwork(t, "testing", context.Background(), net)
	require.NoError(t, err)

	assert.Len(t, flowNetwork.Containers, 4)

	realData := getRealData(flowNetwork)

	expectedData := []data{
		{image: "gcr.io/dl-flow/collection:latest", name: "collection", address: "collection"},
		{image: "gcr.io/dl-flow/consensus:latest", name: "consensus", address: "consensus"},
		{image: "gcr.io/dl-flow/execution:latest", name: "execution", address: "execution"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification", address: "verification"},
	}

	assert.Subset(t, realData, expectedData)
}

func TestNetworkSetupMultipleNodes(t *testing.T) {

	net := []*network.FlowNode{
		{
			Role:  flow.RoleCollection,
			Stake: 1000,
		},
		{
			Role:  flow.RoleCollection,
			Stake: 1000,
		},
		{
			Role:  flow.RoleCollection,
			Stake: 1234,
		},
		{
			Role:  flow.RoleExecution,
			Stake: 7668,
		},
		{
			Role:  flow.RoleVerification,
			Stake: 4582,
		},
		{
			Role:  flow.RoleVerification,
			Stake: 4582,
		},
		{
			Role:  flow.RoleVerification,
			Stake: 4582,
		},
	}

	flowNetwork, err := network.PrepareFlowNetwork(t, "testing", context.Background(), net)
	require.NoError(t, err)

	assert.Len(t, flowNetwork.Containers, 7)

	realData := getRealData(flowNetwork)

	expectedData := []data{
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_0", address: "collection_0"},
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_1", address: "collection_1"},
		{image: "gcr.io/dl-flow/collection:latest", name: "collection_2", address: "collection_2"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_0", address: "verification_0"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_1", address: "verification_1"},
		{image: "gcr.io/dl-flow/verification:latest", name: "verification_2", address: "verification_2"},
		{image: "gcr.io/dl-flow/execution:latest", name: "execution", address: "execution"},
	}

	assert.Subset(t, realData, expectedData)
}

func getRealData(flowNetwork *network.FlowNetwork) []data {
	realData := make([]data, len(flowNetwork.Containers))

	for i, container := range flowNetwork.Containers {
		realData[i] = data{
			image:   container.Image,
			name:    container.Name,
			address: container.Identity.Address,
		}
	}
	return realData
}
