package cmd

import (
	"bytes"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/cmd/bootstrap/run"
	"github.com/dapperlabs/flow-go/crypto"
	model "github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

type collectorType int

const (
	existingInternalCollector collectorType = iota
	existingPartnerCollector
	generatedRandomCollector // a collector that was inserted by the random process
)

type collector struct {
	collectorType  collectorType
	nodeInfoPub    model.NodeInfo
	stakingPrivKey crypto.PrivateKey
}

func (c collector) NodeID() flow.Identifier {
	if c.collectorType == existingInternalCollector || c.collectorType == existingPartnerCollector {
		return c.nodeInfoPub.NodeID
	}
	id, err := flow.PublicKeyToID(c.stakingPrivKey.PublicKey())
	if err != nil {
		log.Fatal().Err(err).Msgf("unable to get NodeID for collector %#v", c)
	}
	return id
}

// generateAdditionalInternalCollectors generates additional internal collectors by generating private staking keys
// until a set of requirements is satisfied. This process is not optimized for performance. The requirements are:
// a) each cluster has at least twice as many internal than external nodes
// b) clusters have to be approx the same size, differing by at most one collector
// c) clusters have as little nodes as possible
// NOTE: this method is probabilistically successful and will fail after a number of maximum tries. This can be fixed
// with a more complex process that shifts the position of partner nodes by deterministically inserting/removing staking
// keys. That more complex approach has not been chosen for simplicity, but depending on the number of tries the current
// approach needs to find a valid collector allocation it might be worthwhile to rething that decision. by shifting
// partner nodes through deterministic
func generateAdditionalInternalCollectors(nClusters, minPerCluster int, internalNodes, partnerNodes []model.NodeInfo) []model.NodeInfo {
	// Check if we need to add any internal nodes at all
	currentTotal := len(internalNodes) + len(partnerNodes)
	// Use floor, since we want the smaller value, e.g. 10 nodes, 3 clusters, would result in cluster sizes of 4, 3, 3
	// we actually want to compare against the worst case, which would be 3
	nodesPerClusterFloor := currentTotal / nClusters
	if len(partnerNodes) < nodesPerClusterFloor/3 {
		return []model.NodeInfo{}
	}

	maxPartnerPerCluster := len(partnerNodes) / nClusters
	if len(partnerNodes)%nClusters > 0 {
		maxPartnerPerCluster++
	}
	nTotal := calcTotalCollectors(nClusters, minPerCluster, len(partnerNodes))

	// require at least 1/3 of collectors to be randomly generated
	if len(internalNodes)+len(partnerNodes) >= nTotal*2/3 {
		log.Fatal().Msgf("at least 1/3 of %v total required collectors should be available for hash grinding, got %v "+
			"internal nodes and %v partner nodes – please reduce the number of internal nodes", nTotal,
			len(internalNodes), len(partnerNodes))
	}

	// list of sorted collectors
	var collectors []collector

	// hash grind until all the requirements are satisfied
hashGrindingLoop:
	for g := 0; ; g++ {
		if g >= int(flagCollectorGenerationMaxHashGrindingIterations) {
			log.Fatal().Msgf("hash grinding to generate internal collectors that satisfy the requirements failed "+
				"after %v iterations", g)
		}

		collectors = make([]collector, 0)
		collectors = insertNodes(collectors, internalNodes, partnerNodes)
		collectors = insertRandomCollectorsSorted(collectors, nTotal-len(collectors))

		nPerCluster := make([]int, nClusters)
		partnersPerCluster := make([]int, nClusters)
		for i, c := range collectors {
			cluster := clusterForIndex(i, nClusters)
			nPerCluster[cluster]++
			if c.collectorType == existingPartnerCollector {
				partnersPerCluster[cluster]++
			}
		}

		// check requirements
		for i := 0; i < nClusters; i++ {
			if partnersPerCluster[i] > maxPartnerPerCluster {
				log.Debug().Msgf("hash grinding iteration %v: too many partner collectors in cluster %v: have %v, "+
					"max %v", g, i, partnersPerCluster[i], maxPartnerPerCluster)
				continue hashGrindingLoop
			}

			if nPerCluster[i] < partnersPerCluster[i]*3 {
				log.Debug().Msgf("hash grinding iteration %v: too few total collectors in cluster %v: have %v, "+
					"min %v*3", g, i, nPerCluster[i], partnersPerCluster[i])
				continue hashGrindingLoop
			}
		}

		break hashGrindingLoop
	}

	// NOTE: the following is a test whether the code above created an assignment that satisfies our requirements.
	err := verifyCollectorClustering(nClusters, minPerCluster, collectors)
	if err != nil {
		log.Fatal().Err(err).Msgf("collector clustering failed")
	}

	return assembleAdditionalInternalCollectors(collectors)
}

func calcTotalCollectors(nClusters, minPerCluster, nPartners int) int {
	maxPartnersPerCluster := nPartners / nClusters
	if nPartners%nClusters > 0 {
		maxPartnersPerCluster++
	}

	maxTotalPerCluster := maxPartnersPerCluster * 3

	maxTotal := maxTotalPerCluster * nClusters

	nClustersWithoutMax := 0
	if nPartners%nClusters > 0 {
		nClustersWithoutMax = nClusters - nPartners%nClusters
	}

	nTotal := maxTotal - nClustersWithoutMax

	if nTotal < minPerCluster*nClusters {
		return minPerCluster * nClusters
	}
	return nTotal
}

func insertNodes(cs []collector, internalNodes, partnerNodes []model.NodeInfo) []collector {
	for _, n := range internalNodes {
		cs = append(cs, collector{
			collectorType: existingInternalCollector,
			nodeInfoPub:   n,
		})
	}
	for _, n := range partnerNodes {
		cs = append(cs, collector{
			collectorType: existingPartnerCollector,
			nodeInfoPub:   n,
		})
	}
	return cs
}

func insertRandomCollectorsSorted(cs []collector, n int) []collector {
	for i := 0; i < n; i++ {
		priv, err := run.GenerateStakingKey(generateRandomSeed())
		if err != nil {
			log.Fatal().Err(err).Msgf("unable to generate staking key")
		}

		cs = append(cs, collector{
			collectorType:  generatedRandomCollector,
			stakingPrivKey: priv,
		})
	}

	sortCollectors(cs)

	return cs
}

func assembleAdditionalInternalCollectors(cs []collector) []model.NodeInfo {

	nodeInfos := make([]model.NodeInfo, 0)
	for _, c := range cs {
		if c.collectorType == existingPartnerCollector || c.collectorType == existingInternalCollector {
			continue
		}

		networkKey, err := run.GenerateNetworkingKey(generateRandomSeed())
		if err != nil {
			log.Fatal().Err(err).Msg("cannot generate networking key")
		}

		nodeConfig := model.NodeConfig{
			Role:    flow.RoleCollection,
			Address: fmt.Sprintf(flagGeneratedCollectorAddressTemplate, len(nodeInfos)),
			Stake:   flagGeneratedCollectorStake,
		}
		nodeInfo := assembleNodeInfo(nodeConfig, networkKey, c.stakingPrivKey)

		nodeInfos = append(nodeInfos, nodeInfo)
	}

	return nodeInfos
}

func sortCollectors(cs []collector) {
	sort.Slice(cs, func(i, j int) bool {
		idI := cs[i].NodeID()
		idJ := cs[j].NodeID()
		return bytes.Compare(idI[:], idJ[:]) < 0
	})
}

func verifyCollectorClustering(nClusters int, minPerCluster int, cs []collector) error {
	sortCollectors(cs)

	nPerCluster := make([]int, nClusters)
	partnersPerCluster := make([]int, nClusters)
	for i, c := range cs {
		cluster := clusterForIndex(i, nClusters)
		nPerCluster[cluster]++
		if c.collectorType == existingPartnerCollector {
			partnersPerCluster[cluster]++
		}
	}

	for i := 0; i < nClusters; i++ {
		if nPerCluster[i] < minPerCluster {
			return fmt.Errorf("need at least %v collectors per cluster, cluster %v only has %v", minPerCluster, i,
				nPerCluster[i])
		}

		if nPerCluster[i] < partnersPerCluster[i]*3 {
			return fmt.Errorf("each cluster needs 3 times the total collectors of the partner collectors, cluster %v "+
				"has %v total collectors and %v partner collectors", i, nPerCluster[i], partnersPerCluster[i])
		}
	}
	return nil
}

// clusterForIndex returns the cluster for the given index in the slice of collector nodes, sorted by NodeID
func clusterForIndex(i, nClusters int) int {
	return i % nClusters
}
