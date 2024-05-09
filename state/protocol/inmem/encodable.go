package inmem

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// EncodableSnapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	LatestSeal          *flow.Seal            // TODO replace with same info from sealing segment
	LatestResult        *flow.ExecutionResult // TODO replace with same info from sealing segment
	SealingSegment      *flow.SealingSegment
	QuorumCertificate   *flow.QuorumCertificate
	Params              EncodableParams
	SealedVersionBeacon *flow.SealedVersionBeacon
}

// Head returns the latest finalized header of the Snapshot.
func (snap EncodableSnapshot) Head() *flow.Header {
	return snap.SealingSegment.Highest().Header
}

func (snap EncodableSnapshot) getLatestSeal() *flow.Seal {
	head := snap.Head()
	latestSealID := snap.SealingSegment.LatestSeals[head.ID()]
	_ = latestSealID
	// iterate backward through payloads of snap.SealingSegment.Blocks
	// search for seal with matching ID
	return nil
}

func (snap EncodableSnapshot) getLatestResult() *flow.ExecutionResult {
	latestSeal := snap.getLatestSeal()
	_ = latestSeal
	// iterate backward through payloads of snap.SealingSegment.Blocks
	// search for result with ID matching latestSeal.ResultID
	return nil
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
