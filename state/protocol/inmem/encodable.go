package inmem

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// EncodableSnapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	Head                *flow.Header
	LatestSeal          *flow.Seal
	LatestResult        *flow.ExecutionResult
	SealingSegment      *flow.SealingSegment
	QuorumCertificate   *flow.QuorumCertificate
	Epochs              EncodableEpochs
	Params              EncodableParams
	EpochProtocolState  *flow.ProtocolStateEntry
	KVStore             storage.KeyValueStoreData
	SealedVersionBeacon *flow.SealedVersionBeacon
}

// EncodableEpochs is the encoding format for protocol.EpochQuery
type EncodableEpochs struct {
	Previous *EncodableEpoch
	Current  EncodableEpoch // cannot be nil
	Next     *EncodableEpoch
}

// EncodableEpoch is the encoding format for protocol.Epoch
type EncodableEpoch struct {
	Counter            uint64
	FirstView          uint64
	DKGPhase1FinalView uint64
	DKGPhase2FinalView uint64
	DKGPhase3FinalView uint64
	FinalView          uint64
	RandomSource       []byte
	InitialIdentities  flow.IdentitySkeletonList
	Clustering         flow.ClusterList
	Clusters           []EncodableCluster
	DKG                *EncodableDKG
	FirstHeight        *uint64
	FinalHeight        *uint64
}

// EncodableDKG is the encoding format for protocol.DKG
type EncodableDKG struct {
	GroupKey     encodable.RandomBeaconPubKey
	Participants map[flow.Identifier]flow.DKGParticipant
}

type EncodableFullDKG struct {
	GroupKey      encodable.RandomBeaconPubKey
	PrivKeyShares []encodable.RandomBeaconPrivKey
	PubKeyShares  []encodable.RandomBeaconPubKey
}

// EncodableCluster is the encoding format for protocol.Cluster
type EncodableCluster struct {
	Index     uint
	Counter   uint64
	Members   flow.IdentitySkeletonList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}

// EncodableParams is the encoding format for protocol.GlobalParams
type EncodableParams struct {
	ChainID                    flow.ChainID
	SporkID                    flow.Identifier
	SporkRootBlockHeight       uint64
	ProtocolVersion            uint
	EpochCommitSafetyThreshold uint64
}
