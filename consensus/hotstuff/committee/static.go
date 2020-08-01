package committee

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NewStaticCommittee returns a new committee with a static participant set.
func NewStaticCommittee(participants flow.IdentityList, myID flow.Identifier, dkgParticipants map[flow.Identifier]epoch.DKGParticipant, dkgGroupKey crypto.PublicKey) (*Static, error) {
	static := &Static{
		participants:    participants,
		myID:            myID,
		dkgParticipants: dkgParticipants,
		dkgGroupKey:     dkgGroupKey,
	}
	return static, nil
}

// Static represents a committee with a static participant set. It is used for
// bootstrapping purposes.
type Static struct {
	participants    flow.IdentityList
	myID            flow.Identifier
	dkgParticipants map[flow.Identifier]epoch.DKGParticipant
	dkgGroupKey     crypto.PublicKey
}

func (s Static) Identities(_ flow.Identifier, selector flow.IdentityFilter) (flow.IdentityList, error) {
	return s.participants.Filter(selector), nil
}

func (s Static) Identity(_ flow.Identifier, participantID flow.Identifier) (*flow.Identity, error) {
	identity, ok := s.participants.ByNodeID(participantID)
	if !ok {
		return nil, fmt.Errorf("unknown partipant")
	}
	return identity, nil
}

func (s Static) LeaderForView(view uint64) (flow.Identifier, error) {
	return flow.ZeroID, fmt.Errorf("invalid for static committee")
}

func (s Static) Self() flow.Identifier {
	return s.myID
}

func (s Static) DKGSize(_ flow.Identifier) (uint, error) {
	return uint(len(s.dkgParticipants)), nil
}

func (s Static) DKGGroupKey(blockID flow.Identifier) (crypto.PublicKey, error) {
	return s.dkgGroupKey, nil
}

func (s Static) DKGIndex(_ flow.Identifier, nodeID flow.Identifier) (uint, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return 0, fmt.Errorf("could not get participant")
	}
	return participant.Index, nil
}

func (s Static) DKGKeyShare(_ flow.Identifier, nodeID flow.Identifier) (crypto.PublicKey, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return nil, fmt.Errorf("could not get participant")
	}
	return participant.KeyShare, nil
}
