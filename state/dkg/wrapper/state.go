package wrapper

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
)

// State implements an isolated DKG state that lives separately from the protocol
// state.
type State struct {
	data *dkg.PublicData
}

// NewState creates a new DKG state based on a DKG data struct.
func NewState(data *dkg.PublicData) *State {
	s := &State{
		data: data,
	}
	return s
}

// GroupSize returns the DKG group size.
func (s *State) GroupSize() (uint, error) {
	return uint(len(s.data.IDToParticipant)), nil
}

// GroupKey returns the public DKG group key.
func (s *State) GroupKey() (crypto.PublicKey, error) {
	return s.data.GroupPubKey, nil
}

// HasParticipant checks if the given node is part of the DKG participants.
func (s *State) HasParticipant(nodeID flow.Identifier) (bool, error) {
	_, exists := s.data.IDToParticipant[nodeID]
	return exists, nil
}

// ParticipantIndex returns the index of the DKG share for the given node.
func (s *State) ParticipantIndex(nodeID flow.Identifier) (uint, error) {
	participant, found := s.data.IDToParticipant[nodeID]
	if !found {
		return 0, fmt.Errorf("DKG participant not found (node: %x)", nodeID)
	}
	return participant.Index, nil
}

// ParticipantKey returns the key share for the given node.
func (s *State) ParticipantKey(nodeID flow.Identifier) (crypto.PublicKey, error) {
	participant, found := s.data.IDToParticipant[nodeID]
	if !found {
		return nil, fmt.Errorf("DKG participant not found (node: %x)", nodeID)
	}
	return participant.PublicKeyShare, nil
}
