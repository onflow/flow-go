package protocol

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
)

// ChainIDForCluster returns the canonical chain ID for a collection node cluster.
// TODO remove
func ChainIDForCluster(cluster flow.IdentityList) flow.ChainID {
	return flow.ChainID(cluster.Fingerprint().String())
}

// CanonicalClusterRootBlock returns the canonical root block for the given
// cluster in the given epoch.
func CanonicalClusterRootBlock(epoch uint64, participants flow.IdentityList) *cluster.Block {

	chainID := fmt.Sprintf("cluster-%d-%s", epoch, participants.Fingerprint())
	payload := cluster.EmptyPayload(flow.ZeroID)
	header := &flow.Header{
		ChainID:        flow.ChainID(chainID),
		ParentID:       flow.ZeroID,
		Height:         0,
		PayloadHash:    payload.Hash(),
		Timestamp:      flow.GenesisTime,
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	return &cluster.Block{
		Header:  header,
		Payload: &payload,
	}
}
