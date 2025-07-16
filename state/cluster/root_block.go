package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// CanonicalClusterID returns the canonical chain ID for the given cluster in
// the given epoch.
func CanonicalClusterID(epoch uint64, participants flow.IdentifierList) flow.ChainID {
	return flow.ChainID(fmt.Sprintf("cluster-%d-%s", epoch, participants.ID()))
}

// these globals are filled by the static initializer
var rootBlockPayload = cluster.NewEmptyPayload(flow.ZeroID)

// CanonicalRootBlock returns the canonical root block for the given
// cluster in the given epoch. It contains an empty collection referencing
func CanonicalRootBlock(epoch uint64, participants flow.IdentitySkeletonList) (*cluster.Block, error) {
	chainID := CanonicalClusterID(epoch, participants.NodeIDs())
	rootHeaderBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            chainID,
		ParentID:           flow.ZeroID,
		Height:             0,
		Timestamp:          uint64(flow.GenesisTime.UnixMilli()),
		View:               0,
		ParentView:         0,
		ParentVoterIndices: nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create root header body: %w", err)
	}

	return cluster.NewRootBlock(
		cluster.UntrustedBlock{
			Header:  *rootHeaderBody,
			Payload: *rootBlockPayload,
		},
	), nil
}
