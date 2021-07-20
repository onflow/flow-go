package epochs

import (
	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// EpochConfig is a placeholder for config values used to deploy the epochs
// smart-contract.
type EpochConfig struct {
	EpochTokenPayout             cadence.UFix64
	RewardCut                    cadence.UFix64
	CurrentEpochCounter          cadence.UInt64
	NumViewsInEpoch              cadence.UInt64
	NumViewsInStakingAuction     cadence.UInt64
	NumViewsInDKGPhase           cadence.UInt64
	NumCollectorClusters         cadence.UInt16
	FLOWsupplyIncreasePercentage cadence.UFix64
	RandomSource                 cadence.String
	CollectorClusters            flow.AssignmentList
	ClusterQCs                   []*flow.QuorumCertificate
	DKGPubKeys                   []crypto.PublicKey
}

// DefaultEpochConfig returns an EpochConfig with default values used for
// testing.
func DefaultEpochConfig() EpochConfig {
	return EpochConfig{
		CurrentEpochCounter:          cadence.UInt64(0),
		NumViewsInEpoch:              cadence.UInt64(100),
		NumViewsInStakingAuction:     cadence.UInt64(10),
		NumViewsInDKGPhase:           cadence.UInt64(10),
		NumCollectorClusters:         cadence.UInt16(3),
		FLOWsupplyIncreasePercentage: cadence.UFix64(5),
	}
}

// ConvertClusterAssignments converts an assignment list into a cadence value
// representing the clusterWeights transaction argument for the deployEpoch
// transaction used during execution state bootstrapping and for the resetEpoch
// transaction used to manually transition between epochs.
//
// The resulting argument has type [{String: UInt64}] which represents a list
// of weight mappings for each cluster. The full Cluster struct is constructed
// within the transaction in Cadence for simplicity here.
//
func ConvertClusterAssignments(clusterAssignments flow.AssignmentList) cadence.Value {

	weightMappingPerCluster := []cadence.Value{}
	for _, cluster := range clusterAssignments {

		weightsByNodeID := []cadence.KeyValuePair{}
		for _, id := range cluster {
			cdcNodeID, err := cadence.NewString(id.String())
			if err != nil {
				panic(err)
			}
			kvp := cadence.KeyValuePair{
				Key:   cdcNodeID,
				Value: cadence.NewUInt64(1),
			}
			weightsByNodeID = append(weightsByNodeID, kvp)
		}

		weightMappingPerCluster = append(weightMappingPerCluster, cadence.NewDictionary(weightsByNodeID))
	}

	return cadence.NewArray(weightMappingPerCluster)
}
