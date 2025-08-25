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

	rootBlockPayload, err := cluster.NewRootPayload(
		cluster.UntrustedPayload(*cluster.NewEmptyPayload(flow.ZeroID)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root cluster payload: %w", err)
	}

	block, err := cluster.NewRootBlock(
		cluster.UntrustedBlock{
			HeaderBody: *rootHeaderBody,
			Payload:    *rootBlockPayload,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create root cluster block: %w", err)
	}

	return block, nil
}
