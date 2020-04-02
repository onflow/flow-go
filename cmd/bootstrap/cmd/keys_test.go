package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"

	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TestClusterFor(t *testing.T) {
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(1), 2))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(2), 2))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(3), 2))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(4), 2))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(5), 2))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(6), 2))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(1), 3))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(2), 3))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(3), 3))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(4), 3))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(5), 3))
	assert.Equal(t, uint(2), clusterFor(flow.MakeID(6), 3))
	assert.Equal(t, uint(1), clusterFor(flow.MakeID(7), 3))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(8), 3))
	assert.Equal(t, uint(2), clusterFor(flow.MakeID(9), 3))
	assert.Equal(t, uint(0), clusterFor(flow.MakeID(10), 3))
}

func TestCalcInternalCollectorNodes(t *testing.T) {
	internalNodes := []model.NodeInfo{
		{
			Role:   flow.RoleConsensus, // should be ignored
			NodeID: flow.MakeID(1),
		},
		{
			Role:   flow.RoleCollection,
			NodeID: flow.MakeID(3),
		},
	}
	partnerNodes := []model.NodeInfo{
		{
			Role:   flow.RoleConsensus, // should be ignored
			NodeID: flow.MakeID(2),
		},
		{
			Role:   flow.RoleCollection,
			NodeID: flow.MakeID(4),
		},
		{
			Role:   flow.RoleCollection,
			NodeID: flow.MakeID(5),
		},
	}
	assert.Equal(t, []int{3, 3}, calcAdditionalCollectorNodes(2, 3, internalNodes, partnerNodes))
}
