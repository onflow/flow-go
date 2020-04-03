// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
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

// NodeInfo contains information for a node. This is used during the bootstrapping
// process to represent each node. When writing node information to disk, use
// `Public` or `Private` to obtain canonical structures.
//
// A NodeInfo instance can contain EITHER public keys OR private keys, not both.
// This can be ensured by using only using the provided constructors and NOT
// manually constructing an instance.
type NodeInfo struct {
	NodeID         flow.Identifier
	Role           flow.Role
	Address        string
	Stake          uint64
	networkPubKey  crypto.PublicKey
	networkPrivKey crypto.PrivateKey
	stakingPubKey  crypto.PublicKey
	stakingPrivKey crypto.PrivateKey
}

func NewNodeInfoWithPublicKeys(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	stake uint64,
	networkKey crypto.PublicKey,
	stakingKey crypto.PublicKey,
) NodeInfo {
	return NodeInfo{
		NodeID:        nodeID,
		Role:          role,
		Address:       addr,
		Stake:         stake,
		networkPubKey: networkKey,
		stakingPubKey: stakingKey,
	}
}

func NewNodeInfoWithPrivateKeys(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	stake uint64,
	networkKey crypto.PrivateKey,
	stakingKey crypto.PrivateKey,
) NodeInfo {
	return NodeInfo{
		NodeID:         nodeID,
		Role:           role,
		Address:        addr,
		Stake:          stake,
		networkPrivKey: networkKey,
		stakingPrivKey: stakingKey,
	}
}

func (node NodeInfo) HasPrivateKeys() bool {
	return node.networkPrivKey != nil && node.stakingPrivKey != nil
}

func (node NodeInfo) HasPublicKeys() bool {
	return node.networkPubKey != nil && node.stakingPubKey != nil
}

func (node NodeInfo) NetworkPubKey() crypto.PublicKey {
	if node.networkPubKey != nil {
		return node.networkPubKey
	}
	return node.networkPrivKey.PublicKey()
}

func (node NodeInfo) NetworkPrivKey() crypto.PrivateKey {
	if !node.HasPrivateKeys() {
		panic("node has no private key info")
	}
	return node.networkPrivKey
}

func (node NodeInfo) StakingPubKey() crypto.PublicKey {
	if node.stakingPubKey != nil {
		return node.stakingPubKey
	}
	return node.stakingPrivKey.PublicKey()
}

func (node NodeInfo) StakingPrivKey() crypto.PrivateKey {
	if !node.HasPrivateKeys() {
		panic("node has no private key info")
	}
	return node.stakingPrivKey
}

func (node NodeInfo) Private() NodeInfoPriv {
	if !node.HasPrivateKeys() {
		panic("node has no private key info")
	}
	return NodeInfoPriv{
		Role:           node.Role,
		Address:        node.Address,
		NodeID:         node.NodeID,
		NetworkPrivKey: EncodableNetworkPrivKey{PrivateKey: node.NetworkPrivKey()},
		StakingPrivKey: EncodableStakingPrivKey{PrivateKey: node.StakingPrivKey()},
	}
}

// Public returns the canonical public structure
func (node NodeInfo) Public() NodeInfoPub {
	return NodeInfoPub{
		Role:          node.Role,
		Address:       node.Address,
		NodeID:        node.NodeID,
		Stake:         node.Stake,
		NetworkPubKey: EncodableNetworkPubKey{PublicKey: node.NetworkPubKey()},
		StakingPubKey: EncodableStakingPubKey{PublicKey: node.StakingPubKey()},
	}
}

// Identity returns the node info as a public Flow identity.
func (node NodeInfo) Identity() *flow.Identity {
	identity := &flow.Identity{
		NodeID:        node.NodeID,
		Address:       node.Address,
		Role:          node.Role,
		Stake:         node.Stake,
		StakingPubKey: node.StakingPubKey(),
		NetworkPubKey: node.NetworkPubKey(),
	}
	return identity
}

func FilterByRole(nodes []NodeInfo, role flow.Role) []NodeInfo {
	var filtered []NodeInfo
	for _, node := range nodes {
		if node.Role != role {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}
