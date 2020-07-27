package run

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/state/protocol"
)

func GenerateRootClusterBlocks(clusters flow.ClusterList) []*cluster.Block {
	clusterBlocks := make([]*cluster.Block, len(clusters))
	for i := range clusterBlocks {
		cluster, ok := clusters.ByIndex(uint(i))
		if !ok {
			panic(fmt.Sprintf("failed to get cluster by index: %v", i))
		}

		clusterBlocks[i] = GenerateRootClusterBlock(cluster)
	}
	return clusterBlocks
}

func GenerateRootClusterBlock(identities flow.IdentityList) *cluster.Block {
	payload := cluster.EmptyPayload(flow.ZeroID)
	header := &flow.Header{
		ChainID:        protocol.ChainIDForCluster(identities),
		ParentID:       flow.ZeroID,
		Height:         0,
		PayloadHash:    payload.Hash(),
		Timestamp:      flow.GenesisTime,
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
