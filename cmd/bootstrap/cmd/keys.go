package cmd

import (
	"fmt"
	"math/big"

	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

type NodeConfig struct {
	Role    flow.Role
	Address string
	Stake   uint64
}

type NodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

type NodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
	Stake         uint64
}

func genNetworkAndStakingKeys(partnerNodes []NodeInfoPub) ([]NodeInfoPub, []NodeInfoPriv) {
	var nodeConfigs []NodeConfig
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

	nodeInfosPub := make([]NodeInfoPub, 0, nodes)
	nodeInfosPriv := make([]NodeInfoPriv, 0, nodes)
	for i, nodeConfig := range nodeConfigs {
		log.Debug().Int("i", i).Str("address", nodeConfig.Address).Msg("assembling node information")
		nodeInfoPriv, nodeInfoPub := assembleNodeInfo(nodeConfig, networkKeys[i], stakingKeys[i])
		nodeInfosPub = append(nodeInfosPub, nodeInfoPub)
		nodeInfosPriv = append(nodeInfosPriv, nodeInfoPriv)
	}

	log.Debug().Msgf("will calculated additionally needed collector nodes to have majority in each cluster")
	additionalCollectorNodes := calcAdditionalCollectorNodes(int(flagCollectionClusters), int(minNodesPerCluster),
		nodeInfosPub, partnerNodes)

	// for each cluster
	i := 0
	for clusterIndex, nAdditionalNodes := range additionalCollectorNodes {
		log.Debug().Msgf("will generate %v additional internal nodes for cluster index %v", nAdditionalNodes,
			clusterIndex)
		// for each node
		for n := 0; n < nAdditionalNodes; n++ {
			stakingKey, err := generateStakingKeyInCluster(clusterIndex, int(flagCollectionClusters))
			if err != nil {
				log.Fatal().Err(err).Msg("cannot generate staking key")
			}
			networkKey, err := run.GenerateNetworkingKey(generateRandomSeed())
			if err != nil {
				log.Fatal().Err(err).Msg("cannot generate networking key")
			}

			nodeInfoPriv, nodeInfoPub := assembleNodeInfo(NodeConfig{
				Role:    flow.RoleCollection,
				Address: fmt.Sprintf(flagGeneratedCollectorAddressTemplate, i),
				Stake:   flagGeneratedCollectorStake,
			}, networkKey, stakingKey)
			nodeInfosPub = append(nodeInfosPub, nodeInfoPub)
			nodeInfosPriv = append(nodeInfosPriv, nodeInfoPriv)

			i++
		}
	}
	log.Debug().Msgf("generated %v additional internal nodes for collection clusters", i)

	for _, nodeInfoPriv := range nodeInfosPriv {
		writeJSON(fmt.Sprintf(filenameNodeInfoPriv, nodeInfoPriv.NodeID), nodeInfoPriv)
	}

	return nodeInfosPub, nodeInfosPriv
}

func assembleNodeInfo(nodeConfig NodeConfig, networkKey, stakingKey crypto.PrivateKey) (NodeInfoPriv, NodeInfoPub) {
	nodeID, err := flow.PublicKeyToID(stakingKey.PublicKey())
	if err != nil {
		log.Fatal().Err(err).Msg("cannot generate NodeID from PublicKey")
	}

	log.Debug().
		Str("networkPubKey", pubKeyToString(networkKey.PublicKey())).
		Str("stakingPubKey", pubKeyToString(stakingKey.PublicKey())).
		Msg("encoded public staking and network keys")

	nodeInfoPriv := NodeInfoPriv{
		Role:           nodeConfig.Role,
		Address:        nodeConfig.Address,
		NodeID:         nodeID,
		NetworkPrivKey: EncodableNetworkPrivKey{networkKey},
		StakingPrivKey: EncodableStakingPrivKey{stakingKey},
	}

	nodeInfoPub := NodeInfoPub{
		Role:          nodeConfig.Role,
		Address:       nodeConfig.Address,
		NodeID:        nodeID,
		NetworkPubKey: EncodableNetworkPubKey{networkKey.PublicKey()},
		StakingPubKey: EncodableStakingPubKey{stakingKey.PublicKey()},
		Stake:         nodeConfig.Stake,
	}

	return nodeInfoPriv, nodeInfoPub
}

func validateAddressesUnique(ns []NodeConfig) {
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
func calcAdditionalCollectorNodes(nClusters int, minPerCluster int, internalNodes, partnerNodes []NodeInfoPub) []int {
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
	for i := 0; i < nClusters; i++ {
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
func clusterFor(nodeID flow.Identifier, nClusters int) int {
	cluster := big.NewInt(0).Mod(big.NewInt(0).SetBytes(nodeID[:]), big.NewInt(int64(nClusters)))
	const maxUint = ^uint(0)
	const maxInt = int(maxUint >> 1)
	if !cluster.IsInt64() || cluster.Int64() > int64(maxInt) {
		log.Fatal().Str("NodeID", nodeID.String()).Int("nClusters", nClusters).Msg("unable to assign cluster")
	}
	return int(cluster.Int64())
}

// generateStakingKeyInCluster creates staking keys until it finds one that belongs to the given clusterIndex
// TODO add timeout?
func generateStakingKeyInCluster(clusterIndex, nClusters int) (crypto.PrivateKey, error) {
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
