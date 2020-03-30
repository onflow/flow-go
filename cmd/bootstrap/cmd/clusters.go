package cmd

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

func computeCollectorClusters(stakingNodes []NodeInfoPub) *flow.ClusterList {
	clusters := flow.NewClusterList(uint(flagCollectionClusters))

	for _, node := range stakingNodes {
		if node.Role != flow.RoleCollection {
			continue
		}

		index := clusterFor(node.NodeID, uint(flagCollectionClusters))
		clusters.Add(uint(index), &flow.Identity{
			NodeID:             node.NodeID,
			Address:            node.Address,
			Role:               node.Role,
			Stake:              node.Stake,
			StakingPubKey:      node.StakingPubKey,
			RandomBeaconPubKey: nil,
			NetworkPubKey:      node.NetworkPubKey,
		})
	}

	return clusters
}

func constructGenesisBlocksForCollectorClusters() []cluster.Block {
	clusterBlocks := run.GenerateGenesisClusterBlocks(int(flagCollectionClusters))

	for i, clusterBlock := range clusterBlocks {
		writeJSON(fmt.Sprintf(filenameGenesisClusterBlock, i), clusterBlock)
	}

	return clusterBlocks
}

func constructGenesisQCsForCollectorClusters(clusterList *flow.ClusterList, nodeInfosPriv []NodeInfoPriv,
	block flow.Block, clusterBlocks []cluster.Block) {
	if len(clusterBlocks) != clusterList.Size() {
		log.Fatal().Int("len(clusterBlocks)", len(clusterBlocks)).Int("clusterList.Size()", clusterList.Size()).
			Msg("number of clusters needs to equal number of cluster blocks")
	}

	for i := 0; i < clusterList.Size(); i++ {
		identities := clusterList.ByIndex(uint(i))

		signerData := createClusterSigners(identities, nodeInfosPriv)

		qc, err := run.GenerateClusterGenesisQC(signerData, &block, &clusterBlocks[i])
		if err != nil {
			log.Fatal().Err(err).Int("cluster index", i).Msg("generating collector cluster genesis QC failed")
		}

		writeJSON(fmt.Sprintf(filenameGenesisClusterQC, i), qc)
	}
}

func createClusterSigners(identities flow.IdentityList, nodeInfosPriv []NodeInfoPriv) []run.ClusterSigner {
	clusterSigners := make([]run.ClusterSigner, 0, len(identities))
	for _, identity := range identities {
		found, pk := findNodeInfoPriv(nodeInfosPriv, identity.NodeID)
		if !found {
			log.Debug().Msg("could not find private key for collector, skipping it as a signer")
			continue
		}
		clusterSigners = append(clusterSigners, run.ClusterSigner{
			Identity:       *identity,
			StakingPrivKey: pk.StakingPrivKey,
		})
	}
	return clusterSigners
}

func findNodeInfoPriv(nsPriv []NodeInfoPriv, nodeID flow.Identifier) (bool, NodeInfoPriv) {
	for _, nPriv := range nsPriv {
		if nPriv.NodeID == nodeID {
			return true, nPriv
		}
	}
	return false, NodeInfoPriv{}
}
