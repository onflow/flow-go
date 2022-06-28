package cluster

import (
	"fmt"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// CanonicalClusterID returns the canonical chain ID for the given cluster in
// the given epoch.
func CanonicalClusterID(epoch uint64, participants flow.IdentityList) flow.ChainID {
	return flow.ChainID(fmt.Sprintf("cluster-%d-%s", epoch, participants.Fingerprint()))
}

// these globals are filled by the static initializer
var rootBlockPayload = cluster.EmptyPayload(flow.ZeroID)
var rootBlockPayloadHash = rootBlockPayload.Hash()

// CanonicalRootBlock returns the canonical root block for the given
// cluster in the given epoch. It contains an empty collection referencing
func CanonicalRootBlock(epoch uint64, participants flow.IdentityList) *cluster.Block {
	chainID := CanonicalClusterID(epoch, participants)

	header := flow.NewHeader(
		chainID,
		flow.ZeroID,
		0,
		rootBlockPayloadHash,
		flow.GenesisTime,
		0,
		nil,
		nil,
		flow.ZeroID,
		nil)

	return &cluster.Block{
		Header:  header,
		Payload: &rootBlockPayload,
	}
}
