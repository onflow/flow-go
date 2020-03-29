package bootstrap

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/dkg"
	"github.com/dapperlabs/flow-go/model/flow"
)

// DKGParticipantPriv is the canonical structure for encoding private node DKG information.
type DKGParticipantPriv struct {
	NodeID              flow.Identifier
	RandomBeaconPrivKey EncodableRandomBeaconPrivKey
	GroupIndex          int
}

// DKGParticipantPub is the canonical structure for encoding public node DKG information.
type DKGParticipantPub struct {
	NodeID             flow.Identifier
	RandomBeaconPubKey EncodableRandomBeaconPubKey
	GroupIndex         int
}

// DKGParticipant represents a DKG participant, including private information.
// It is used while running the DKG during boostrapping. To write public DKG
// data, or per-node key-shares, use `Private` or `Public` for the appropriate
// canonical encoding.
type DKGParticipant struct {
	NodeID     flow.Identifier
	KeyShare   crypto.PrivateKey
	GroupIndex int
}

// Private returns the canonical private structure.
func (part *DKGParticipant) Private() DKGParticipantPriv {
	return DKGParticipantPriv{
		NodeID:              part.NodeID,
		RandomBeaconPrivKey: EncodableRandomBeaconPrivKey{PrivateKey: part.KeyShare},
		GroupIndex:          part.GroupIndex,
	}
}

// Public returns the canonical public structure.
func (part *DKGParticipant) Public() DKGParticipantPub {
	return DKGParticipantPub{
		NodeID:             part.NodeID,
		RandomBeaconPubKey: EncodableRandomBeaconPubKey{PublicKey: part.KeyShare.PublicKey()},
		GroupIndex:         part.GroupIndex,
	}
}

// DKGData represents all data in the DKG process, including private information.
// It is used while running the DKG during boostrapping.
type DKGData struct {
	PubGroupKey  crypto.PublicKey
	Participants []DKGParticipant
}

// DKGDataPub is canonical structure for encoding public DKG data.
type DKGDataPub struct {
	PubGroupKey  EncodableRandomBeaconPubKey
	Participants []DKGParticipantPub
}

// Public returns the canonical public structure.
func (dd *DKGData) Public() DKGDataPub {

	pub := DKGDataPub{
		PubGroupKey:  EncodableRandomBeaconPubKey{PublicKey: dd.PubGroupKey},
		Participants: make([]DKGParticipantPub, 0, len(dd.Participants)),
	}

	for _, part := range dd.Participants {
		pub.Participants = append(pub.Participants, part.Public())
	}

	return pub
}

// ForHotStuff converts DKG data for use in HotStuff.
func (dd *DKGData) ForHotStuff() *dkg.PublicData {

	participantLookup := make(map[flow.Identifier]*dkg.Participant)
	for _, part := range dd.Participants {
		participantLookup[part.NodeID] = &dkg.Participant{
			PublicKeyShare: part.KeyShare.PublicKey(),
			Index:          uint(part.GroupIndex),
		}
	}

	return &dkg.PublicData{
		GroupPubKey:     dd.PubGroupKey,
		IDToParticipant: participantLookup,
	}
}
