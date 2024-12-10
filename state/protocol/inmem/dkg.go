package inmem

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// DKG defines a new type using flow.EpochCommit as underlying type and implements the protocol.DKG interface.
type DKG flow.EpochCommit

var _ protocol.DKG = (*DKG)(nil)

// NewDKG creates a new DKG instance from the given setup and commit events.
// TODO(EFM, #6794): Remove this once we complete the network upgrade, we should remove v0 model.
func NewDKG(setup *flow.EpochSetup, commit *flow.EpochCommit) protocol.DKG {
	if commit.DKGIndexMap == nil {
		return NewDKGv0(setup, commit)
	} else {
		return (*DKG)(commit)
	}
}

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
	return flow.ZeroID, fmt.Errorf("inconsistent DKG state: missing index %d", index)
}

// DKGv0 implements the protocol.DKG interface for the model which is currently active on mainnet.
// This model is used for [flow.EpochCommit] events without the DKGIndexMap field.
// TODO(EFM, #6794): Remove this once we complete the network upgrade
type DKGv0 struct {
	Participants flow.IdentitySkeletonList
	Commit       *flow.EpochCommit
}

var _ protocol.DKG = (*DKGv0)(nil)

func NewDKGv0(setup *flow.EpochSetup, commit *flow.EpochCommit) *DKGv0 {
	return &DKGv0{
		Participants: setup.Participants.Filter(filter.IsConsensusCommitteeMember),
		Commit:       commit,
	}
}

func (d DKGv0) Size() uint {
	return uint(len(d.Participants))
}

func (d DKGv0) GroupKey() crypto.PublicKey {
	return d.Commit.DKGGroupKey
}

// Index returns the DKG index for the given node.
// Expected error during normal operations:
//   - protocol.IdentityNotFoundError if nodeID is not a known DKG participant
func (d DKGv0) Index(nodeID flow.Identifier) (uint, error) {
	index, exists := d.Participants.GetIndex(nodeID)
	if !exists {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return index, nil
}

// KeyShare returns the public key share for the given node.
// Expected error during normal operations:
//   - protocol.IdentityNotFoundError if nodeID is not a known DKG participant
func (d DKGv0) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	index, err := d.Index(nodeID)
	if err != nil {
		return nil, err
	}
	return d.Commit.DKGParticipantKeys[index], nil
}

// NodeID returns the node identifier for the given index.
// An exception is returned if the index is ≥ Size().
// Intended for use outside the hotpath, with runtime
// scaling linearly in the number of DKG participants (ie. Size())
func (d DKGv0) NodeID(index uint) (flow.Identifier, error) {
	identity, exists := d.Participants.ByIndex(index)
	if !exists {
		return flow.ZeroID, fmt.Errorf("inconsistent DKG state: missing index %d", index)
	}
	return identity.NodeID, nil
}
