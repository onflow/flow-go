// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"encoding/json"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NodeInfoType enumerates the two different options for
type NodeInfoType int

const (
	NodeInfoTypeInvalid NodeInfoType = iota
	NodeInfoTypePublic
	NodeInfoTypePrivate
)

// ErrMissingPrivateInfo is returned when a method is called on NodeInfo
// that is only valid on instances containing private info.
var ErrMissingPrivateInfo = fmt.Errorf("can not access private information for a public node type")

// NodeConfig contains configuration information used as input to the
// bootstrap process.
type NodeConfig struct {
	Role    flow.Role
	Address string
	Stake   uint64
}

// Defines the canonical structure for encoding private node info.
type NodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey EncodableNetworkPrivKey
	StakingPrivKey EncodableStakingPrivKey
}

func (node *NodeInfoPriv) Encode() ([]byte, error) {
	return json.MarshalIndent(node, "", "  ")
}

func DecodeNodeInfoPriv(raw []byte, target interface{}) error {
	return json.Unmarshal(raw, target)
}

// Defines the canonical structure for encoding public node info.
type NodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	Stake         uint64
	NetworkPubKey EncodableNetworkPubKey
	StakingPubKey EncodableStakingPubKey
}

func (node *NodeInfoPub) Encode() ([]byte, error) {
	return json.MarshalIndent(node, "", "  ")
}

func DecodeNodeInfoPub(raw []byte, target interface{}) error {
	return json.Unmarshal(raw, target)
}

// NodePrivateKeys is a wrapper for the private keys for a node, comprising all
// sensitive information for a node.
type NodePrivateKeys struct {
	StakingKey crypto.PrivateKey
	NetworkKey crypto.PrivateKey
}

// NodeInfo contains information for a node. This is used during the bootstrapping
// process to represent each node. When writing node information to disk, use
// `Public` or `Private` to obtain the appropriate canonical structure.
//
// A NodeInfo instance can contain EITHER public keys OR private keys, not both.
// This can be ensured by using only using the provided constructors and NOT
// manually constructing an instance.
type NodeInfo struct {
	NodeID  flow.Identifier
	Role    flow.Role
	Address string
	Stake   uint64

	// key information is private
	networkPubKey  crypto.PublicKey
	networkPrivKey crypto.PrivateKey
	stakingPubKey  crypto.PublicKey
	stakingPrivKey crypto.PrivateKey
}

func NewPublicNodeInfo(
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

func NewPrivateNodeInfo(
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

// Type returns the type of the node info instance.
func (node NodeInfo) Type() NodeInfoType {
	if node.networkPrivKey != nil && node.stakingPrivKey != nil {
		return NodeInfoTypePrivate
	}
	if node.networkPubKey != nil && node.stakingPubKey != nil {
		return NodeInfoTypePublic
	}
	return NodeInfoTypeInvalid
}

func (node NodeInfo) NetworkPubKey() crypto.PublicKey {
	if node.networkPubKey != nil {
		return node.networkPubKey
	}
	return node.networkPrivKey.PublicKey()
}

func (node NodeInfo) StakingPubKey() crypto.PublicKey {
	if node.stakingPubKey != nil {
		return node.stakingPubKey
	}
	return node.stakingPrivKey.PublicKey()
}

func (node NodeInfo) PrivateKeys() (*NodePrivateKeys, error) {
	if node.Type() != NodeInfoTypePrivate {
		return nil, ErrMissingPrivateInfo
	}
	return &NodePrivateKeys{
		StakingKey: node.stakingPrivKey,
		NetworkKey: node.networkPrivKey,
	}, nil
}

// Private returns the canonical private encodable structure.
func (node NodeInfo) Private() (NodeInfoPriv, error) {
	if node.Type() != NodeInfoTypePrivate {
		return NodeInfoPriv{}, ErrMissingPrivateInfo
	}

	return NodeInfoPriv{
		Role:           node.Role,
		Address:        node.Address,
		NodeID:         node.NodeID,
		NetworkPrivKey: EncodableNetworkPrivKey{PrivateKey: node.networkPrivKey},
		StakingPrivKey: EncodableStakingPrivKey{PrivateKey: node.stakingPrivKey},
	}, nil
}

// Public returns the canonical public encodable structure
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

// PartnerPublic returns the public data for a partner node.
func (node NodeInfo) PartnerPublic() PartnerNodeInfoPub {
	return PartnerNodeInfoPub{
		Role:          node.Role,
		Address:       node.Address,
		NodeID:        node.NodeID,
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

func ToIdentityList(nodes []NodeInfo) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(nodes))
	for _, node := range nodes {
		il = append(il, node.Identity())
	}
	return il
}
