package inmem

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// EncodableSnapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	Head              *flow.Header
	Identities        flow.IdentityList
	LatestSeal        *flow.Seal
	LatestResult      *flow.ExecutionResult
	SealingSegment    []*flow.Block
	QuorumCertificate *flow.QuorumCertificate
	Phase             flow.EpochPhase
	Epochs            EncodableEpochs
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
	InitialIdentities  flow.IdentityList
	Clustering         flow.ClusterList
	Clusters           []EncodableCluster
	DKG                *EncodableDKG
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
	Members   flow.IdentityList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}
