// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Defines the canonical structure for private node info.
// NOTE: should be used only for encoding.
type nodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

// Defines the canonical structure for public node info.
// NOTE: should be used only for encoding.
type nodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	Stake         uint64
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
}

// NodeInfo contains configuration information for a node, including private
// information. This is used during the bootstrapping process to represent each
// node. When writing node information to disk, use `Public` or `Private`.
type NodeInfo struct {
	NodeID         flow.Identifier
	Role           flow.Role
	Address        string
	Stake          uint64
	NetworkPrivKey crypto.PrivateKey
	StakingPrivKey crypto.PrivateKey
}

func (node NodeInfo) Private() nodeInfoPriv {
	return nodeInfoPriv{
		Role:           node.Role,
		Address:        node.Address,
		NodeID:         node.NodeID,
		NetworkPrivKey: EncodableNetworkPrivKey{PrivateKey: node.NetworkPrivKey},
		StakingPrivKey: EncodableStakingPrivKey{PrivateKey: node.StakingPrivKey},
	}
}

// Public returns the canonical public structure
func (node NodeInfo) Public() nodeInfoPub {
	return nodeInfoPub{
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

type DKGParticipant struct {
	NodeID     flow.Identifier
	KeyShare   crypto.PrivateKey
	GroupIndex int
}

type DKGData struct {
	GroupPubKey  crypto.PublicKey
	Participants []DKGParticipant
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
		GroupPubKey:           dkg.GroupPubKey,
		IdToDKGParticipantMap: participantLookup,
	}
}
