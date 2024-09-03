package inmem

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

// DKG defines a new type using flow.EpochCommit as underlying type and implements the protocol.DKG interface.
type DKG flow.EpochCommit

var _ protocol.DKG = (*DKG)(nil)

func NewDKG(commit *flow.EpochCommit) *DKG { return (*DKG)(commit) }

func (d *DKG) Size() uint                 { return uint(len(d.DKGParticipantKeys)) }
func (d *DKG) GroupKey() crypto.PublicKey { return d.DKGGroupKey }

// Index returns the DKG index for the given node.
// Expected error during normal operations:
//   - protocol.IdentityNotFoundError if nodeID is not a known DKG participant
func (d *DKG) Index(nodeID flow.Identifier) (uint, error) {
	index, exists := d.DKGIndexMap[nodeID]
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	if index < 0 { // sanity check, to rule out underflow in subsequent conversion to `uint`
		return 0, fmt.Errorf("for node %v, DKGIndexMap contains negative index %d in violation of protocol convention", nodeID, index)
	}
	return uint(index), nil
}

// KeyShare returns the public key share for the given node.
// Expected error during normal operations:
//   - protocol.IdentityNotFoundError if nodeID is not a known DKG participant
func (d *DKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	index, exists := d.DKGIndexMap[nodeID]
	if !exists {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return d.DKGParticipantKeys[index], nil
}

// NodeID returns the node identifier for the given index.
// An exception is returned if the index is ≥ Size().
// Intended for use outside the hotpath, with runtime
// scaling linearly in the number of DKG participants (ie. Size())
func (d *DKG) NodeID(index uint) (flow.Identifier, error) {
	for nodeID, dkgIndex := range d.DKGIndexMap {
		if dkgIndex == int(index) {
			return nodeID, nil
		}
	}
	return flow.ZeroID, fmt.Errorf("index %d not found in DKG", index)
}
