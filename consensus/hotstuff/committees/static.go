package committees

import (
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
)

func NewStaticReplicas(participants flow.IdentitySkeletonList, myID flow.Identifier, dkgParticipants map[flow.Identifier]flow.DKGParticipant, dkgGroupKey crypto.PublicKey) (*StaticReplicas, error) {
	return NewStaticReplicasWithDKG(participants, myID, staticDKG{
		dkgParticipants: dkgParticipants,
		dkgGroupKey:     dkgGroupKey,
	})
}

func NewStaticReplicasWithDKG(participants flow.IdentitySkeletonList, myID flow.Identifier, dkg protocol.DKG) (*StaticReplicas, error) {
	valid := flow.IsIdentityListCanonical(participants)
	if !valid {
		return nil, fmt.Errorf("participants %v is not in Canonical order", participants)
	}

	static := &StaticReplicas{
		participants: participants,
		myID:         myID,
		dkg:          dkg,
	}
	return static, nil
}

// NewStaticCommittee returns a new committee with a static participant set.
func NewStaticCommittee(participants flow.IdentityList, myID flow.Identifier, dkgParticipants map[flow.Identifier]flow.DKGParticipant, dkgGroupKey crypto.PublicKey) (*Static, error) {
	return NewStaticCommitteeWithDKG(participants, myID, staticDKG{
		dkgParticipants: dkgParticipants,
		dkgGroupKey:     dkgGroupKey,
	})
}

// NewStaticCommitteeWithDKG returns a new committee with a static participant set.
func NewStaticCommitteeWithDKG(participants flow.IdentityList, myID flow.Identifier, dkg protocol.DKG) (*Static, error) {
	replicas, err := NewStaticReplicasWithDKG(participants.ToSkeleton(), myID, dkg)
	if err != nil {
		return nil, fmt.Errorf("could not create static replicas: %w", err)
	}

	static := &Static{
		StaticReplicas: *replicas,
		fullIdentities: participants,
	}
	return static, nil
}

type StaticReplicas struct {
	participants flow.IdentitySkeletonList
	myID         flow.Identifier
	dkg          protocol.DKG
}

var _ hotstuff.Replicas = (*StaticReplicas)(nil)

func (s StaticReplicas) IdentitiesByEpoch(view uint64) (flow.IdentitySkeletonList, error) {
	return s.participants.ToSkeleton(), nil
}

func (s StaticReplicas) IdentityByEpoch(view uint64, participantID flow.Identifier) (*flow.IdentitySkeleton, error) {
	identity, ok := s.participants.ByNodeID(participantID)
	if !ok {
		return nil, model.NewInvalidSignerErrorf("unknown participant %x", participantID)
	}
	return identity, nil
}

func (s StaticReplicas) LeaderForView(_ uint64) (flow.Identifier, error) {
	return flow.ZeroID, fmt.Errorf("invalid for static committee")
}

func (s StaticReplicas) QuorumThresholdForView(_ uint64) (uint64, error) {
	return WeightThresholdToBuildQC(s.participants.ToSkeleton().TotalWeight()), nil
}

func (s StaticReplicas) TimeoutThresholdForView(_ uint64) (uint64, error) {
	return WeightThresholdToTimeout(s.participants.ToSkeleton().TotalWeight()), nil
}

func (s StaticReplicas) Self() flow.Identifier {
	return s.myID
}

func (s StaticReplicas) DKG(_ uint64) (hotstuff.DKG, error) {
	return s.dkg, nil
}

// Static represents a committee with a static participant set. It is used for
// bootstrapping purposes.
type Static struct {
	StaticReplicas
	fullIdentities flow.IdentityList
}

var _ hotstuff.DynamicCommittee = (*Static)(nil)

func (s Static) IdentitiesByBlock(_ flow.Identifier) (flow.IdentityList, error) {
	return s.fullIdentities, nil
}

func (s Static) IdentityByBlock(_ flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	identity, ok := s.fullIdentities.ByNodeID(participantID)
	if !ok {
		return nil, model.NewInvalidSignerErrorf("unknown participant %x", participantID)
	}
	return identity, nil
}

type staticDKG struct {
	dkgParticipants map[flow.Identifier]flow.DKGParticipant
	dkgGroupKey     crypto.PublicKey
}

func (s staticDKG) Size() uint {
	return uint(len(s.dkgParticipants))
}

func (s staticDKG) GroupKey() crypto.PublicKey {
	return s.dkgGroupKey
}

// Index returns the index for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (s staticDKG) Index(nodeID flow.Identifier) (uint, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return 0, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return participant.Index, nil
}

// KeyShare returns the public key share for the given node. Error Returns:
// protocol.IdentityNotFoundError if nodeID is not a valid DKG participant.
func (s staticDKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return nil, protocol.IdentityNotFoundError{NodeID: nodeID}
	}
	return participant.KeyShare, nil
}

// KeyShares returns the public portions of all threshold key shares. Note that there might not
// exist a private key corresponding to each entry (e.g. if the respective node failed the DKG).
func (s staticDKG) KeyShares() []crypto.PublicKey {
	participants := make([]crypto.PublicKey, len(s.dkgParticipants))
	for _, participant := range s.dkgParticipants {
		participants[participant.Index] = participant.KeyShare
	}
	return participants
}

// NodeID returns the node identifier for the given index.
// An exception is returned if the index is ≥ Size().
// Intended for use outside the hotpath, with runtime
// scaling linearly in the number of DKG participants (ie. Size())
func (s staticDKG) NodeID(index uint) (flow.Identifier, error) {
	for nodeID, participant := range s.dkgParticipants {
		if participant.Index == index {
			return nodeID, nil
		}
	}
	return flow.ZeroID, fmt.Errorf("index %d not found in DKG", index)
}
