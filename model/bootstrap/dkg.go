package bootstrap

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
)

// DKGData represents all the output data from the DKG process, including private information.
// It is used while running the DKG during bootstrapping.
type DKGData struct {
	PrivKeyShares []crypto.PrivateKey
	PubGroupKey   crypto.PublicKey
	PubKeyShares  []crypto.PublicKey
}

// bootstrap.DKGParticipantPriv is the canonical structure for encoding private node DKG information.
type DKGParticipantPriv struct {
	NodeID              flow.Identifier
	RandomBeaconPrivKey EncodableRandomBeaconPrivKey
	GroupIndex          int
}

// DKGParticipantPub is the canonical structure for encoding public node DKG information.
type EncodableDKGParticipantPub struct {
	NodeID             flow.Identifier
	RandomBeaconPubKey EncodableRandomBeaconPubKey
	GroupIndex         int
}

// DKGDataPub is canonical structure for encoding public DKG data.
type EncodableDKGDataPub struct {
	PubGroupKey  EncodableRandomBeaconPubKey
	Participants []EncodableDKGParticipantPub
}

// Public returns the canonical public structure.
func (dd *DKGData) Public(nodes []NodeInfo) EncodableDKGDataPub {

	pub := EncodableDKGDataPub{
		PubGroupKey:  EncodableRandomBeaconPubKey{PublicKey: dd.PubGroupKey},
		Participants: make([]EncodableDKGParticipantPub, 0, len(dd.PubKeyShares)),
	}

	for i, pk := range dd.PubKeyShares {
		nodeID := nodes[i].NodeID
		encPk := EncodableRandomBeaconPubKey{pk}
		pub.Participants = append(pub.Participants, EncodableDKGParticipantPub{
			NodeID:             nodeID,
			RandomBeaconPubKey: encPk,
			GroupIndex:         i,
		})
	}

	return pub
}

// ForHotStuff converts DKG data for use in HotStuff.
func (pub *EncodableDKGDataPub) ForHotStuff() *dkg.PublicData {

	participantLookup := make(map[flow.Identifier]*dkg.Participant)
	for _, participant := range pub.Participants {
		participantLookup[participant.NodeID] = &dkg.Participant{
			PublicKeyShare: participant.RandomBeaconPubKey,
			Index:          uint(participant.GroupIndex),
		}
	}

	return &dkg.PublicData{
		GroupPubKey:     pub.PubGroupKey,
		IDToParticipant: participantLookup,
	}
}
