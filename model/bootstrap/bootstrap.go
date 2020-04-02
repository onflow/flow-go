// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NodeConfig contains configuration information used as input to the
// bootstrap process.
type NodeConfig struct {
	Role    flow.Role
	Address string
	Stake   uint64
}

// Defines the canonical structure for private node info.
// NOTE: should be used only for encoding.
type NodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

// Defines the canonical structure for public node info.
// NOTE: should be used only for encoding.
type NodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	Stake         uint64
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
}

// NodeInfo contains information for a node, including private information.
// This is used during the bootstrapping process to represent each node.
// When writing node information to disk, use `Public` or `Private`.
type NodeInfo struct {
	NodeID         flow.Identifier
	Role           flow.Role
	Address        string
	Stake          uint64
	NetworkPubKey  crypto.PublicKey
	NetworkPrivKey crypto.PrivateKey
	StakingPubKey  crypto.PublicKey
	StakingPrivKey crypto.PrivateKey
}

func (node NodeInfo) HasPrivateKeys() bool {
	return node.NetworkPrivKey != nil && node.StakingPrivKey != nil
}

func (node NodeInfo) Private() NodeInfoPriv {
	return NodeInfoPriv{
		Role:           node.Role,
		Address:        node.Address,
		NodeID:         node.NodeID,
		NetworkPrivKey: EncodableNetworkPrivKey{PrivateKey: node.NetworkPrivKey},
		StakingPrivKey: EncodableStakingPrivKey{PrivateKey: node.StakingPrivKey},
	}
}

// Public returns the canonical public structure
func (node NodeInfo) Public() NodeInfoPub {
	return NodeInfoPub{
		Role:          node.Role,
		Address:       node.Address,
		NodeID:        node.NodeID,
		Stake:         node.Stake,
		NetworkPubKey: EncodableNetworkPubKey{PublicKey: node.NetworkPrivKey.PublicKey()},
		StakingPubKey: EncodableStakingPubKey{PublicKey: node.StakingPrivKey.PublicKey()},
	}
}

// Identity returns the node info as a public Flow identity.
func (node NodeInfo) Identity() *flow.Identity {
	identity := &flow.Identity{
		NodeID:        node.NodeID,
		Address:       node.Address,
		Role:          node.Role,
		Stake:         node.Stake,
		StakingPubKey: node.StakingPrivKey.PublicKey(),
		NetworkPubKey: node.NetworkPrivKey.PublicKey(),
	}
	return identity
}

// NodeInfoFromIdentity converts a public Flow identity to a node info.
func NodeInfoFromIdentity(identity *flow.Identity) NodeInfo {
	return NodeInfo{
		NodeID:         identity.NodeID,
		Role:           identity.Role,
		Address:        identity.Address,
		Stake:          identity.Stake,
		NetworkPrivKey: nil,
		StakingPrivKey: nil,
	}
}

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
