package epochs

import (
	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"

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
		CurrentEpochCounter:      cadence.UInt64(0),
		NumViewsInEpoch:          cadence.UInt64(10_000),
		NumViewsInStakingAuction: cadence.UInt64(1000),
		NumViewsInDKGPhase:       cadence.UInt64(1000),
		//NumViewsInEpoch:              cadence.UInt64(100),
		//NumViewsInStakingAuction:     cadence.UInt64(10),
		//NumViewsInDKGPhase:           cadence.UInt64(10),
		NumCollectorClusters:         cadence.UInt16(3),
		FLOWsupplyIncreasePercentage: cadence.UFix64(5),
	}
}

// EncodeClusterAssignment encodes an AssigmentList into a byte array that can
// be used as a transaction argument when deploying the epochs contract.
func EncodeClusterAssignments(clusterAssignments flow.AssignmentList, service flow.Address) []byte {
	collectorClusterValues := []cadence.Value{}

	for i, cluster := range clusterAssignments {
		clusterIndex := cadence.UInt16(i)

		weightsByNodeID := []cadence.KeyValuePair{}
		for _, id := range cluster {
			kvp := cadence.KeyValuePair{
				Key:   cadence.NewString(id.String()),
				Value: cadence.NewUInt64(1),
			}
			weightsByNodeID = append(weightsByNodeID, kvp)
		}

		totalWeight := cadence.NewUInt64(uint64(len(cluster)))

		votes := cadence.NewArray([]cadence.Value{})

		fields := []cadence.Value{
			clusterIndex,
			cadence.NewDictionary(weightsByNodeID),
			totalWeight,
			votes,
		}

		clusterStruct := cadence.NewStruct(fields).
			WithType(&cadence.StructType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(service.Bytes()),
					Name:    "Service",
				},
				QualifiedIdentifier: "FlowEpochClusterQC.Cluster",
				Fields: []cadence.Field{
					{
						Identifier: "index",
						Type:       cadence.UInt16Type{},
					},
					{
						Identifier: "nodeWeights",
						Type: cadence.DictionaryType{
							KeyType:     cadence.StringType{},
							ElementType: cadence.UInt64Type{},
						},
					},
					{
						Identifier: "totalWeight",
						Type:       cadence.UInt64Type{},
					},
					{
						Identifier: "votes",
						Type: cadence.ConstantSizedArrayType{
							ElementType: cadence.AnyStructType{},
						},
					},
				},
			})

		collectorClusterValues = append(collectorClusterValues, clusterStruct)
	}

	collectorClusters := cadence.NewArray(collectorClusterValues)

	return jsoncdc.MustEncode(collectorClusters)
}

// EncodeClusterQCs encodes a slice of QuorumCertificates into a byte array that
// can be used as a transaction argument when deploying the epochs contract.
func EncodeClusterQCs(qcs []*flow.QuorumCertificate, service flow.Address) []byte {
	qcValues := []cadence.Value{}

	for i, qc := range qcs {
		qcIndex := cadence.UInt16(i)

		// Here we are adding signer IDs rather than votes. It doesn't matter
		// because these initial values aren't used by the contract.
		qcVotes := []cadence.Value{}
		for _, v := range qc.SignerIDs {
			qcVotes = append(qcVotes, cadence.NewString(v.String()))
		}

		fields := []cadence.Value{
			qcIndex,
			cadence.NewArray(qcVotes),
		}

		qcStruct := cadence.NewStruct(fields).
			WithType(&cadence.StructType{
				Location: common.AddressLocation{
					Address: common.BytesToAddress(service.Bytes()),
					Name:    "Service",
				},
				QualifiedIdentifier: "FlowEpochClusterQC.ClusterQC",
				Fields: []cadence.Field{
					{
						Identifier: "index",
						Type:       cadence.UInt16Type{},
					},
					{
						Identifier: "votes",
						Type: cadence.ConstantSizedArrayType{
							ElementType: cadence.StringType{},
						},
					},
				},
			})

		qcValues = append(qcValues, qcStruct)
	}

	quorumCertificates := cadence.NewArray(qcValues)

	return jsoncdc.MustEncode(quorumCertificates)
}

// EncodePubKeys encodes a slice of public keys into a byte array that can be
// used as a transaction argument when deploying the epochs contract.
func EncodePubKeys(pubKeys []crypto.PublicKey, service flow.Address) []byte {
	pubKeyValues := []cadence.Value{}
	for _, pk := range pubKeys {
		pubKeyValues = append(pubKeyValues, cadence.NewString(pk.String()))
	}
	res := cadence.NewArray(pubKeyValues)
	return jsoncdc.MustEncode(res)
}
