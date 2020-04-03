// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO consolidate with bootstrap model and integration NodeConfig
type NodeConfig struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

// TODO consolidate with bootstrap model ( and maybe hotstuff dkg )
type DKGParticipantPub struct {
	ID          flow.Identifier
	PubKeyShare EncodableRandomBeaconPubKey
	GroupIndex  int
}

type DKGParticipantPriv struct {
	ID           flow.Identifier
	PrivKeyShare EncodableRandomBeaconPrivKey
	GroupIndex   int
}

type DKGPubData struct {
	GroupPubKey  EncodableRandomBeaconPubKey
	Participants []DKGParticipantPub
}

func (pub *DKGPubData) ForHotStuff() *hotstuff.DKGPublicData {

	participantLookup := make(map[flow.Identifier]*hotstuff.DKGParticipant)
	for _, part := range pub.Participants {
		participantLookup[part.ID] = &hotstuff.DKGParticipant{
			Id:             part.ID,
			PublicKeyShare: part.PubKeyShare.PublicKey,
			DKGIndex:       part.GroupIndex,
		}
	}

	return &hotstuff.DKGPublicData{
		GroupPubKey:           pub.GroupPubKey.PublicKey,
		IdToDKGParticipantMap: participantLookup,
	}
}
