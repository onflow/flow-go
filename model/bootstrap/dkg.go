package bootstrap

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/encodable"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO remove encodable DKG types

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
	RandomBeaconPrivKey encodable.RandomBeaconPrivKey
	GroupIndex          int
}

// DKGParticipantPub is the canonical structure for encoding public node DKG information.
type EncodableDKGParticipantPub struct {
	NodeID             flow.Identifier
	RandomBeaconPubKey encodable.RandomBeaconPubKey
	GroupIndex         int
}

// DKGDataPub is canonical structure for encoding public DKG data.
type EncodableDKGDataPub struct {
	PubGroupKey  encodable.RandomBeaconPubKey
	Participants []EncodableDKGParticipantPub
}

// Public returns the canonical public structure.
func (dd *DKGData) Public(nodes []NodeInfo) EncodableDKGDataPub {

	pub := EncodableDKGDataPub{
		PubGroupKey:  encodable.RandomBeaconPubKey{PublicKey: dd.PubGroupKey},
		Participants: make([]EncodableDKGParticipantPub, 0, len(dd.PubKeyShares)),
	}

	for i, pk := range dd.PubKeyShares {
		nodeID := nodes[i].NodeID
		encPk := encodable.RandomBeaconPubKey{pk}
		pub.Participants = append(pub.Participants, EncodableDKGParticipantPub{
			NodeID:             nodeID,
			RandomBeaconPubKey: encPk,
			GroupIndex:         i,
		})
	}

	return pub
}
