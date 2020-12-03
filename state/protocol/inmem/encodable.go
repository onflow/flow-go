package inmem

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// Snapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	Head              *flow.Header
	Identities        flow.IdentityList
	Commit            flow.StateCommitment
	QuorumCertificate *flow.QuorumCertificate
	Phase             flow.EpochPhase
	Epochs            EncodableEpochs
}

// EncodableEpochs is the encoding format for protocol.EpochQuery
type EncodableEpochs struct {
	Previous *EncodableEpoch
	Current  *EncodableEpoch
	Next     *EncodableEpoch
}

// Epoch is the encoding format for protocol.Epoch
type EncodableEpoch struct {
	Counter           uint64
	FirstView         uint64
	FinalView         uint64
	RandomSource      []byte
	InitialIdentities flow.IdentityList
	Clusters          []EncodableCluster
	DKG               *EncodableDKG
}

// DKG is the encoding format for protocol.DKG
type EncodableDKG struct {
	GroupKey     encodable.RandomBeaconPubKey
	Participants map[flow.Identifier]flow.DKGParticipant
}

// Cluster is the encoding format for protocol.Cluster
type EncodableCluster struct {
	Index     uint
	Counter   uint64
	Members   flow.IdentityList
	RootBlock *cluster.Block
	RootQC    *flow.QuorumCertificate
}
