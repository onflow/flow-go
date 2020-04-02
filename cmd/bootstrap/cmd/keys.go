package cmd

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/crypto"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

func genNetworkAndStakingKeys(partnerNodes []model.NodeInfo) []model.NodeInfo {

	var nodeConfigs []model.NodeConfig
	readJSON(flagConfig, &nodeConfigs)
	nodes := len(nodeConfigs)
	log.Info().Msgf("read %v node configurations", nodes)

	validateAddressesUnique(nodeConfigs)
	log.Debug().Msg("all node addresses are unique")

	log.Debug().Msgf("will generate %v networking keys for nodes in config", nodes)
	networkKeys, err := run.GenerateNetworkingKeys(nodes, generateRandomSeeds(nodes))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate networking keys")
	}
	log.Info().Msgf("generated %v networking keys for nodes in config", nodes)

	log.Debug().Msgf("will generate %v staking keys for nodes in config", nodes)
	stakingKeys, err := run.GenerateStakingKeys(nodes, generateRandomSeeds(nodes))
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate staking keys")
	}
	log.Info().Msgf("generated %v staking keys for nodes in config", nodes)

	nodeInfos := make([]model.NodeInfo, 0, len(nodeConfigs))
	for i, nodeConfig := range nodeConfigs {
		log.Debug().Int("i", i).Str("address", nodeConfig.Address).Msg("assembling node information")

		nodeInfo := assembleNodeInfo(nodeConfig, networkKeys[i], stakingKeys[i])
		nodeInfos = append(nodeInfos, nodeInfo)
		writeJSON(fmt.Sprintf(FilenameNodeInfoPriv, nodeInfo.NodeID), nodeInfo.Private())
	}

	log.Debug().Msgf("will calculated additionally needed collector nodes to have majority in each cluster")
	additionalCollectorNodes := calcAdditionalCollectorNodes(uint(flagCollectionClusters), int(minNodesPerCluster), nodeInfos, partnerNodes)

	// for each cluster
	i := 0
	for clusterIndex, nAdditionalNodes := range additionalCollectorNodes {
		log.Debug().Msgf("will generate %v additional internal nodes for cluster index %v", nAdditionalNodes,
			clusterIndex)
		// for each node
		for n := 0; n < nAdditionalNodes; n++ {
			stakingKey, err := generateStakingKeyInCluster(uint(clusterIndex), uint(flagCollectionClusters))
			if err != nil {
				log.Fatal().Err(err).Msg("cannot generate staking key")
			}
			networkKey, err := run.GenerateNetworkingKey(generateRandomSeed())
			if err != nil {
				log.Fatal().Err(err).Msg("cannot generate networking key")
			}

			nodeInfo := assembleNodeInfo(model.NodeConfig{
				Role:    flow.RoleCollection,
				Address: fmt.Sprintf(flagGeneratedCollectorAddressTemplate, i),
				Stake:   flagGeneratedCollectorStake,
			}, networkKey, stakingKey)
			nodeInfos = append(nodeInfos, nodeInfo)

			i++
		}
	}
	log.Debug().Msgf("generated %v additional internal nodes for collection clusters", i)

	for _, nodeInfo := range nodeInfos {
		writeJSON(fmt.Sprintf(FilenameNodeInfoPriv, nodeInfo.NodeID), nodeInfo.Private())
	}

	return nodeInfos
}

func assembleNodeInfo(nodeConfig model.NodeConfig, networkKey, stakingKey crypto.PrivateKey) model.NodeInfo {

	nodeID, err := flow.PublicKeyToID(stakingKey.PublicKey())
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate NodeID from PublicKey")
	}

	log.Debug().
		Str("networkPubKey", pubKeyToString(networkKey.PublicKey())).
		Str("stakingPubKey", pubKeyToString(stakingKey.PublicKey())).
		Msg("encoded public staking and network keys")

	nodeInfo := model.NodeInfo{
		NodeID:         nodeID,
		Role:           nodeConfig.Role,
		Address:        nodeConfig.Address,
		Stake:          nodeConfig.Stake,
		NetworkPrivKey: networkKey,
		StakingPrivKey: stakingKey,
	}

	return nodeInfo
}

func validateAddressesUnique(ns []model.NodeConfig) {
	lookup := make(map[string]struct{})
	for _, n := range ns {
		if _, ok := lookup[n.Address]; ok {
			log.Fatal().Str("address", n.Address).Msg("duplicate node address in config")
		}
	}
}

// calcAdditionalCollectorNodes calculates how many additional internal collector nodes need to be generated based on
// the number of clusters and the number of existing nodes. Returns a slice of integers, where each element represents
// one cluster and the value represents how many nodes are needed in that cluster
func calcAdditionalCollectorNodes(nClusters uint, minPerCluster int, internalNodes, partnerNodes []model.NodeInfo) []int {
	partnerNodesByCluster := make([]map[flow.Identifier]struct{}, nClusters)
	for i := range partnerNodesByCluster {
		partnerNodesByCluster[i] = make(map[flow.Identifier]struct{})
	}

	for _, p := range partnerNodes {
		// skip non-collector nodes
		if p.Role != flow.RoleCollection {
			continue
		}

		i := clusterFor(p.NodeID, nClusters)
		partnerNodesByCluster[i][p.NodeID] = struct{}{}
	}

	internalNodesByCluster := make([]map[flow.Identifier]struct{}, nClusters)
	for i := range internalNodesByCluster {
		internalNodesByCluster[i] = make(map[flow.Identifier]struct{})
	}

	for _, p := range internalNodes {
		// skip non-collector nodes
		if p.Role != flow.RoleCollection {
			continue
		}

		i := clusterFor(p.NodeID, nClusters)
		internalNodesByCluster[i][p.NodeID] = struct{}{}
	}

	res := make([]int, nClusters)
	for i := uint(0); i < nClusters; i++ {
		nPartner := len(partnerNodesByCluster[i])
		nRequiredInternal := nPartner * 2
		if nPartner+nRequiredInternal < minPerCluster {
			nRequiredInternal = minPerCluster - nPartner
		}
		nExistingInternal := len(internalNodesByCluster[i])
		nAdditionalInternal := nRequiredInternal - len(internalNodesByCluster[i])
		if nAdditionalInternal < 0 {
			nAdditionalInternal = 0
		}
		log.Debug().Msgf("cluster %v has %v partner nodes, %v existing internal nodes and needs %v additional "+
			"internal nodes", i, nPartner, nExistingInternal, nAdditionalInternal)
		res[i] = nAdditionalInternal
	}
	return res
}

// clusterFor returns the cluster index for the given node
// TODO replace with actual logic
func clusterFor(nodeID flow.Identifier, nClusters uint) uint {
	cluster := big.NewInt(0).Mod(big.NewInt(0).SetBytes(nodeID[:]), big.NewInt(int64(nClusters)))
	const maxUint = ^uint(0)
	const maxInt = int(maxUint >> 1)
	if !cluster.IsInt64() || cluster.Int64() > int64(maxInt) {
		log.Fatal().Str("NodeID", nodeID.String()).Uint("nClusters", nClusters).Msg("unable to assign cluster")
	}
	return uint(cluster.Int64())
}

// generateStakingKeyInCluster creates staking keys until it finds one that belongs to the given clusterIndex
// TODO add timeout?
func generateStakingKeyInCluster(clusterIndex, nClusters uint) (crypto.PrivateKey, error) {
	for {
		k, err := run.GenerateStakingKey(generateRandomSeed())
		if err != nil {
			return nil, err
		}
		id, err := flow.PublicKeyToID(k.PublicKey())
		if err != nil {
			return nil, err
		}
		if clusterFor(id, nClusters) == clusterIndex {
			return k, nil
		}
	}
}
