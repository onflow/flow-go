package inmem

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

type DKG struct {
	commit *flow.EpochCommit
}

var _ protocol.DKG = (*DKG)(nil)

func NewDKG(commit *flow.EpochCommit) *DKG {
	return &DKG{commit: commit}
}

func (d DKG) Size() uint                 { return uint(len(d.commit.DKGParticipantKeys)) }
func (d DKG) GroupKey() crypto.PublicKey { return d.commit.DKGGroupKey }

// Index returns the index for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (d DKG) Index(nodeID flow.Identifier) (uint, error) {
	index, exists := d.commit.DKGIndexMap[nodeID]
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return uint(index), nil
}

// KeyShare returns the public key share for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (d DKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	index, exists := d.commit.DKGIndexMap[nodeID]
	if !exists {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return d.commit.DKGParticipantKeys[index], nil
}

// NodeID returns the node identifier for the given index.
// An exception is returned if the index is >= Size().
func (d DKG) NodeID(index uint) (flow.Identifier, error) {
	for nodeID, dkgIndex := range d.commit.DKGIndexMap {
		if dkgIndex == int(index) {
			return nodeID, nil
		}
	}
	return flow.ZeroID, fmt.Errorf("index %d not found in DKG", index)
}
