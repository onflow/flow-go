package run

import (
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
)

func GenerateGenesisClusterBlocks(clusters *flow.ClusterList) []*cluster.Block {
	clusterBlocks := make([]*cluster.Block, clusters.Size())
	for i := range clusterBlocks {
		clusterBlocks[i] = GenerateGenesisClusterBlock(clusters.ByIndex(uint(i)))
	}
	return clusterBlocks
}

func GenerateGenesisClusterBlock(identities flow.IdentityList) *cluster.Block {
	payload := cluster.EmptyPayload(flow.ZeroID)
	header := &flow.Header{
		ChainID:        protocol.ChainIDForCluster(identities),
		ParentID:       flow.ZeroID,
		Height:         0,
		PayloadHash:    payload.Hash(),
		Timestamp:      flow.GenesisTime(),
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	return &cluster.Block{
		Header:  header,
		Payload: &payload,
	}
}
