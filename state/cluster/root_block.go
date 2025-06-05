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
func CanonicalRootBlock(epoch uint64, participants flow.IdentitySkeletonList) *cluster.Block {
	chainID := CanonicalClusterID(epoch, participants.NodeIDs())

	headerBody := flow.HeaderBody{
		ChainID:            chainID,
		ParentID:           flow.ZeroID,
		Height:             0,
		Timestamp:          flow.GenesisTime,
		View:               0,
		ParentVoterIndices: nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
	}
	block := cluster.NewBlock(headerBody, rootBlockPayload)

	return &block
}
