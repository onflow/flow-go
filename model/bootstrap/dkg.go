package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Defines the canonical structure for private node DKG information.
// NOTE: should only be used for encoding.
type DKGParticipantPriv struct {
	NodeID              flow.Identifier
	RandomBeaconPrivKey EncodableRandomBeaconPrivKey
	GroupIndex          int
}

// Defines the canonical structure for public node DKG information.
// NOTE: should only be used for encoding.
type DKGParticipantPub struct {
	NodeID             flow.Identifier
	RandomBeaconPubKey EncodableRandomBeaconPubKey
	GroupIndex         int
}

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

type DKGData struct {
	PubGroupKey  crypto.PublicKey
	Participants []DKGParticipant
}

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
