package cluster

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// CanonicalClusterID returns the canonical chain ID for the given cluster in
// the given epoch.
func CanonicalClusterID(epoch uint64, participants flow.IdentityList) flow.ChainID {
	return flow.ChainID(fmt.Sprintf("cluster-%d-%s", epoch, participants.Fingerprint()))
}

var cachedHash flow.Identifier
var once sync.Once

// CanonicalRootBlock returns the canonical root block for the given
// cluster in the given epoch. It contains an empty collection referencing
func CanonicalRootBlock(epoch uint64, participants flow.IdentityList) *cluster.Block {

	chainID := CanonicalClusterID(epoch, participants)
	payload := cluster.EmptyPayload(flow.ZeroID)

	once.Do(func() {
		cachedHash = payload.Hash()
	})
	header := &flow.Header{
		ChainID:            chainID,
		ParentID:           flow.ZeroID,
		Height:             0,
		PayloadHash:        cachedHash,
		Timestamp:          flow.GenesisTime,
		View:               0,
		ParentVoterIDs:     nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
		ProposerSigData:    nil,
	}

	return &cluster.Block{
		Header:  header,
		Payload: &payload,
	}
}
