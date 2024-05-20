package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

// EncodableSnapshot is the encoding format for protocol.Snapshot
type EncodableSnapshot struct {
	SealingSegment      *flow.SealingSegment
	QuorumCertificate   *flow.QuorumCertificate
	Params              EncodableParams
	SealedVersionBeacon *flow.SealedVersionBeacon
}

// Head returns the latest finalized header of the Snapshot, which is the block
// in the sealing segment with the greatest Height.
// The EncodableSnapshot receiver must be correctly formed.
func (snap EncodableSnapshot) Head() *flow.Header {
	return snap.SealingSegment.Highest().Header
}

// LatestSeal returns the latest seal of the Snapshot. This is the seal
// for the block with the greatest height, of all seals in the Snapshot.
// The EncodableSnapshot receiver must be correctly formed.
// No errors are expected during normal operation.
func (snap EncodableSnapshot) LatestSeal() (*flow.Seal, error) {
	head := snap.Head()
	latestSealID := snap.SealingSegment.LatestSeals[head.ID()]

	// CASE 1: The spork root block is the latest sealed block.
	// By protocol definition, FirstSeal seals the spork root block.
	if snap.SealingSegment.FirstSeal != nil && snap.SealingSegment.FirstSeal.ID() == latestSealID {
		return snap.SealingSegment.FirstSeal, nil
	}

	// CASE 2: For any other snapshot, the latest seal must be in a block payload.
	// Since seals are included in increasing height order, the latest seal must be in the
	// first block (by height descending) which contains any seals.
	for i := len(snap.SealingSegment.Blocks) - 1; i >= 0; i-- {
		block := snap.SealingSegment.Blocks[i]
		for _, seal := range block.Payload.Seals {
			if seal.ID() == latestSealID {
				return seal, nil
			}
		}
		if len(block.Payload.Seals) > 0 {
			// We encountered a block with some seals, but not the latest seal.
			// This can only occur in a structurally invalid SealingSegment.
			return nil, fmt.Errorf("LatestSeal: sanity check failed: no latest seal")
		}
	}
	// Correctly formatted sealing segments must contain latest seal.
	return nil, fmt.Errorf("LatestSeal: unreachable for correctly formatted sealing segments")
}

// LatestSealedResult returns the latest sealed result of the Snapshot.
// This is the result which is sealed by LatestSeal.
// The EncodableSnapshot receiver must be correctly formed.
// No errors are expected during normal operation.
func (snap EncodableSnapshot) LatestSealedResult() (*flow.ExecutionResult, error) {
	latestSeal, err := snap.LatestSeal()
	if err != nil {
		return nil, fmt.Errorf("LatestSealedResult: could not get latest seal: %w", err)
	}

	// For both spork root and mid-spork snapshots, the latest sealing result must
	// either appear in a block payload or in the ExecutionResults field.
	for i := len(snap.SealingSegment.Blocks) - 1; i >= 0; i-- {
		block := snap.SealingSegment.Blocks[i]
		for _, result := range block.Payload.Results {
			if latestSeal.ResultID == result.ID() {
				return result, nil
			}
		}
	}
	for _, result := range snap.SealingSegment.ExecutionResults {
		if latestSeal.ResultID == result.ID() {
			return result, nil
		}
	}
	// Correctly formatted sealing segments must contain latest result.
	return nil, fmt.Errorf("LatestSealedResult: unreachable for correctly formatted sealing segments")
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
