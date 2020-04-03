package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
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
func (dkg *DKGData) Public() DKGDataPub {

	pub := DKGDataPub{
		PubGroupKey:  EncodableRandomBeaconPubKey{PublicKey: dkg.PubGroupKey},
		Participants: make([]DKGParticipantPub, 0, len(dkg.Participants)),
	}

	for _, part := range dkg.Participants {
		pub.Participants = append(pub.Participants, part.Public())
	}

	return pub
}

// ForHotStuff converts DKG data for use in HotStuff.
func (dkg *DKGData) ForHotStuff() *hotstuff.DKGPublicData {

	participantLookup := make(map[flow.Identifier]*hotstuff.DKGParticipant)
	for _, part := range dkg.Participants {
		participantLookup[part.NodeID] = &hotstuff.DKGParticipant{
			Id:             part.NodeID,
			PublicKeyShare: part.KeyShare.PublicKey(),
			DKGIndex:       part.GroupIndex,
		}
	}

	return &hotstuff.DKGPublicData{
		GroupPubKey:           dkg.PubGroupKey,
		IdToDKGParticipantMap: participantLookup,
	}
}
