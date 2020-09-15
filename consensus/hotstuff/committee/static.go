package committee

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NewStaticCommittee returns a new committee with a static participant set.
func NewStaticCommittee(participants flow.IdentityList, myID flow.Identifier, dkgParticipants map[flow.Identifier]flow.DKGParticipant, dkgGroupKey crypto.PublicKey) (*Static, error) {
	static := &Static{
		participants: participants,
		myID:         myID,
		dkg: staticDKG{
			dkgParticipants: dkgParticipants,
			dkgGroupKey:     dkgGroupKey,
		},
	}
	return static, nil
}

// Static represents a committee with a static participant set. It is used for
// bootstrapping purposes.
type Static struct {
	participants flow.IdentityList
	myID         flow.Identifier
	dkg          staticDKG
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

func (s Static) LeaderForView(_ uint64) (flow.Identifier, error) {
	return flow.ZeroID, fmt.Errorf("invalid for static committee")
}

func (s Static) Self() flow.Identifier {
	return s.myID
}

func (s Static) DKG(_ flow.Identifier) (hotstuff.DKG, error) {
	return s.dkg, nil
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

func (s staticDKG) Index(nodeID flow.Identifier) (uint, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return 0, fmt.Errorf("could not get participant")
	}
	return participant.Index, nil
}

func (s staticDKG) KeyShare(nodeID flow.Identifier) (crypto.PublicKey, error) {
	participant, ok := s.dkgParticipants[nodeID]
	if !ok {
		return nil, fmt.Errorf("could not get participant")
	}
	return participant.KeyShare, nil
}
