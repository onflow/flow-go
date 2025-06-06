// Package bootstrap defines canonical models and encoding for bootstrapping.
package bootstrap

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onflow/crypto"
	"golang.org/x/exp/slices"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMachineAccountSignAlgo        = sdkcrypto.ECDSA_P256
	DefaultMachineAccountHashAlgo        = sdkcrypto.SHA3_256
	DefaultMachineAccountKeyIndex uint32 = 0
)

// ErrMissingPrivateInfo is returned when a method is called on NodeInfo
// that is only valid on instances containing private info.
var ErrMissingPrivateInfo = fmt.Errorf("can not access private information for a public node type")

// NodeMachineAccountKey contains the private configration need to construct a
// NodeMachineAccountInfo object. This is used as an intemediary by the bootstrap scripts
// for storing the private key before generating a NodeMachineAccountInfo.
type NodeMachineAccountKey struct {
	PrivateKey encodable.MachineAccountPrivKey
}

// NodeMachineAccountInfo defines the structure for a bootstrapping file containing
// private information about the node's machine account. The machine account is used
// by the protocol software to interact with Flow as a client autonomously as needed, in
// particular to run the DKG and generate root cluster quorum certificates when preparing
// for an epoch.
type NodeMachineAccountInfo struct {
	// Address is the flow address of the machine account, not to be confused
	// with the network address of the node.
	Address string

	// EncodedPrivateKey is the private key of the machine account
	EncodedPrivateKey []byte

	// KeyIndex is the index of the key in the associated machine account
	KeyIndex uint32

	// SigningAlgorithm is the algorithm used by the machine account along with
	// the above private key to create cryptographic signatures
	SigningAlgorithm sdkcrypto.SignatureAlgorithm

	// HashAlgorithm is the algorithm used for hashing
	HashAlgorithm sdkcrypto.HashAlgorithm
}

func (info NodeMachineAccountInfo) FlowAddress() flow.Address {
	// trim 0x-prefix if present
	addr := info.Address
	if strings.ToLower(addr[:2]) == "0x" {
		addr = addr[2:]
	}
	return flow.HexToAddress(addr)
}

func (info NodeMachineAccountInfo) SDKAddress() sdk.Address {
	flowAddr := info.FlowAddress()
	var sdkAddr sdk.Address
	copy(sdkAddr[:], flowAddr[:])
	return sdkAddr
}

func (info NodeMachineAccountInfo) PrivateKey() (crypto.PrivateKey, error) {
	sk, err := crypto.DecodePrivateKey(info.SigningAlgorithm, info.EncodedPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("could not decode machine account private key: %w", err)
	}
	return sk, nil
}

func (info NodeMachineAccountInfo) MustPrivateKey() crypto.PrivateKey {
	sk, err := info.PrivateKey()
	if err != nil {
		panic(err)
	}
	return sk
}

// NodeConfig contains configuration information used as input to the
// bootstrap process.
type NodeConfig struct {
	// Role is the flow role of the node (ex Collection, Consensus, ...)
	Role flow.Role

	// Address is the networking address of the node (IP:PORT), not to be
	// confused with the address of the flow account associated with the node's
	// machine account.
	Address string

	// Weight is the weight of the node
	Weight uint64
}

// decodableNodeConfig provides backward-compatible decoding of old models
// which use the Stake field in place of Weight.
type decodableNodeConfig struct {
	Role    flow.Role
	Address string
	Weight  uint64
	// Stake previously was used in place of the Weight field.
	Stake uint64
}

func (conf *NodeConfig) UnmarshalJSON(b []byte) error {
	var decodable decodableNodeConfig
	err := json.Unmarshal(b, &decodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	// compat: translate Stake fields to Weight
	if decodable.Stake != 0 {
		if decodable.Weight != 0 {
			return fmt.Errorf("invalid NodeConfig with both Stake and Weight fields")
		}
		decodable.Weight = decodable.Stake
	}
	conf.Role = decodable.Role
	conf.Address = decodable.Address
	conf.Weight = decodable.Weight
	return nil
}

// NodeInfoPriv defines the canonical structure for encoding private node info.
type NodeInfoPriv struct {
	role           flow.Role
	Address        string
	nodeID         flow.Identifier
	Weight         uint64
	NetworkPrivKey encodable.NetworkPrivKey
	StakingPrivKey encodable.StakingPrivKey
}

// encodableNodeInfoPriv provides encoding/decoding methods that include the
// required fields of `NodeInfoPriv`, including the private fields (private fields aren't accessible to
// JSON write)
type encodableNodeInfoPriv struct {
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey encodable.NetworkPrivKey
	StakingPrivKey encodable.StakingPrivKey
}

// NodeInfoPub defines the canonical structure for encoding public node info.
type NodeInfoPub struct {
	role          flow.Role
	Address       string
	nodeID        flow.Identifier
	Weight        uint64
	NetworkPubKey encodable.NetworkPubKey
	StakingPubKey encodable.StakingPubKey
	StakingPoP    encodable.StakingKeyPoP
}

// encodableNodeInfoPub provides the canonical structure for encoding public node info
type encodableNodeInfoPub struct {
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	Weight        uint64
	NetworkPubKey encodable.NetworkPubKey
	StakingPubKey encodable.StakingPubKey
	StakingPoP    encodable.StakingKeyPoP
}

// decodableNodeInfoPub provides backward-compatible decoding of old models
// which use the Stake field in place of Weight.
type decodableNodeInfoPub struct {
	encodableNodeInfoPub

	// Stake previously was used in place of the Weight field.
	// Deprecated: supported in decoding for backward-compatibility
	Stake uint64
}

func (info *NodeInfoPriv) MarshalJSON() ([]byte, error) {
	enc := encodableNodeInfoPriv{
		Role:           info.role,
		Address:        info.Address,
		NodeID:         info.nodeID,
		NetworkPrivKey: info.NetworkPrivKey,
		StakingPrivKey: info.StakingPrivKey,
	}
	return json.Marshal(enc)
}

func (info *NodeInfoPriv) UnmarshalJSON(b []byte) error {
	var dec encodableNodeInfoPriv
	err := json.Unmarshal(b, &dec)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	info.role = dec.Role
	info.Address = dec.Address
	info.nodeID = dec.NodeID
	info.NetworkPrivKey = dec.NetworkPrivKey
	info.StakingPrivKey = dec.StakingPrivKey
	return nil
}

func (info *NodeInfoPub) Equals(other *NodeInfoPub) bool {
	if other == nil {
		return false
	}
	return info.Address == other.Address &&
		info.nodeID == other.nodeID &&
		info.role == other.role &&
		info.Weight == other.Weight &&
		info.NetworkPubKey.PublicKey.Equals(other.NetworkPubKey.PublicKey) &&
		info.StakingPubKey.PublicKey.Equals(other.StakingPubKey.PublicKey) &&
		slices.Equal(info.StakingPoP.Signature, other.StakingPoP.Signature)
}

func (info *NodeInfoPub) MarshalJSON() ([]byte, error) {
	enc := encodableNodeInfoPub{
		Role:          info.role,
		Address:       info.Address,
		NodeID:        info.nodeID,
		Weight:        info.Weight,
		NetworkPubKey: info.NetworkPubKey,
		StakingPubKey: info.StakingPubKey,
		StakingPoP:    info.StakingPoP,
	}
	return json.Marshal(enc)
}

func (info *NodeInfoPub) UnmarshalJSON(b []byte) error {
	var decodable decodableNodeInfoPub
	err := json.Unmarshal(b, &decodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	// compat: translate Stake fields to Weight
	if decodable.Stake != 0 {
		if decodable.Weight != 0 {
			return fmt.Errorf("invalid NodeInfoPub with both Stake and Weight fields")
		}
		decodable.Weight = decodable.Stake
	}
	info.role = decodable.Role
	info.Address = decodable.Address
	info.nodeID = decodable.NodeID
	info.Weight = decodable.Weight
	info.NetworkPubKey = decodable.NetworkPubKey
	info.StakingPubKey = decodable.StakingPubKey
	info.StakingPoP = decodable.StakingPoP
	return nil
}

// NodePrivateKeys is a wrapper for the private keys for a node, comprising all
// sensitive information for a node.
type NodePrivateKeys struct {
	StakingKey crypto.PrivateKey
	NetworkKey crypto.PrivateKey
}

type NodeInfo interface {
	NodeID() flow.Identifier
	Identity() *flow.Identity
	Role() flow.Role
}

var _ NodeInfo = NodeInfoPriv{}
var _ NodeInfo = NodeInfoPub{}

func NewPublicNodeInfo(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	weight uint64,
	networkKey crypto.PublicKey,
	stakingKey crypto.PublicKey,
	stakingPoP crypto.Signature,
) NodeInfoPub {
	return NodeInfoPub{
		nodeID:        nodeID,
		role:          role,
		Address:       addr,
		Weight:        weight,
		NetworkPubKey: encodable.NetworkPubKey{networkKey},
		StakingPubKey: encodable.StakingPubKey{stakingKey},
		StakingPoP:    encodable.StakingKeyPoP{stakingPoP},
	}
}

func NewPrivateNodeInfo(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	weight uint64,
	networkKey crypto.PrivateKey,
	stakingKey crypto.PrivateKey,
) NodeInfoPriv {
	return NodeInfoPriv{
		nodeID:         nodeID,
		role:           role,
		Address:        addr,
		Weight:         weight,
		NetworkPrivKey: encodable.NetworkPrivKey{PrivateKey: networkKey},
		StakingPrivKey: encodable.StakingPrivKey{PrivateKey: stakingKey},
	}
}

func (node NodeInfoPriv) PrivateKeys() (*NodePrivateKeys, error) {
	return &NodePrivateKeys{
		StakingKey: node.StakingPrivKey,
		NetworkKey: node.NetworkPrivKey,
	}, nil
}

// Public returns the canonical encodable structure holding the node's public information.
// It derives the networking and staking public keys, as well as the Proof of Possession (PoP) of the staking private key
// if they are not already provided in the NodeInfo.
//
// It errors, if there is a problem generating the staking key PoP.
func (node NodeInfoPriv) Public() (NodeInfoPub, error) {
	pop, err := crypto.BLSGeneratePOP(node.StakingPrivKey.PrivateKey)
	if err != nil {
		return NodeInfoPub{}, fmt.Errorf("staking PoP generation failed: %w", err)
	}

	return NodeInfoPub{
		role:          node.role,
		Address:       node.Address,
		nodeID:        node.nodeID,
		Weight:        node.Weight,
		NetworkPubKey: encodable.NetworkPubKey{PublicKey: node.NetworkPrivKey.PublicKey()},
		StakingPubKey: encodable.StakingPubKey{PublicKey: node.StakingPrivKey.PublicKey()},
		StakingPoP:    encodable.StakingKeyPoP{Signature: pop},
	}, nil
}

// PartnerPublic returns the public data for a partner node.
func (node NodeInfoPub) PartnerPublic() (PartnerNodeInfoPub, error) {
	return PartnerNodeInfoPub{
		Role:          node.role,
		Address:       node.Address,
		NodeID:        node.nodeID,
		NetworkPubKey: node.NetworkPubKey,
		StakingPubKey: node.StakingPubKey,
		StakingPoP:    node.StakingPoP,
	}, nil
}

// Identity returns the node info as a public Flow identity.
func (node NodeInfoPub) Identity() *flow.Identity {
	identity := &flow.Identity{
		IdentitySkeleton: flow.IdentitySkeleton{
			NodeID:        node.nodeID,
			Address:       node.Address,
			Role:          node.role,
			InitialWeight: node.Weight,
			StakingPubKey: node.StakingPubKey.PublicKey,
			NetworkPubKey: node.NetworkPubKey.PublicKey,
		},
		DynamicIdentity: flow.DynamicIdentity{
			EpochParticipationStatus: flow.EpochParticipationStatusActive,
		},
	}
	return identity
}

// Identity returns the node info as a public Flow identity.
func (node NodeInfoPriv) Identity() *flow.Identity {
	pub, err := node.Public()
	if err != nil {
		return nil
	}
	return pub.Identity()
}

func (node NodeInfoPub) Role() flow.Role {
	return node.role
}

func (node NodeInfoPriv) Role() flow.Role {
	return node.role
}

func (node NodeInfoPub) NodeID() flow.Identifier {
	return node.nodeID
}

func (node NodeInfoPriv) NodeID() flow.Identifier {
	return node.nodeID
}

func (node NodeInfoPriv) NetworkPubKey() crypto.PublicKey {
	return node.NetworkPrivKey.PrivateKey.PublicKey()
}

// PrivateNodeInfoPubFromIdentity builds a NodeInfo from a flow Identity.
// WARNING: Nothing enforces that the output NodeInfo's keys are corresponding to the input Identity.
func PrivateNodeInfoPubFromIdentity(identity *flow.Identity, networkKey, stakingKey crypto.PrivateKey) NodeInfoPriv {
	return NewPrivateNodeInfo(
		identity.NodeID,
		identity.Role,
		identity.Address,
		identity.InitialWeight,
		networkKey,
		stakingKey,
	)
}

func FilterByRole(nodes []NodeInfo, role flow.Role) []NodeInfo {
	var filtered []NodeInfo
	for _, node := range nodes {
		if node.Role() != role {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

func FilterPrivateByRole(nodes []NodeInfoPriv, role flow.Role) []NodeInfoPriv {
	var filtered []NodeInfoPriv
	for _, node := range nodes {
		if node.Role() != role {
			continue
		}
		filtered = append(filtered, node)
	}
	return filtered
}

// Sort sorts the NodeInfo list using the given ordering.
//
// The sorted list is returned and the original list is untouched.
func Sort(nodes []NodeInfo, order flow.IdentityOrder[flow.Identity]) []NodeInfo {
	dup := make([]NodeInfo, len(nodes))
	copy(dup, nodes)
	slices.SortFunc(dup, func(i, j NodeInfo) int {
		return order(i.Identity(), j.Identity())
	})
	return dup
}

// SortPrivate sorts the NodeInfoPriv list using the given ordering.
//
// The sorted list is returned and the original list is untouched.
func SortPrivate(nodes []NodeInfoPriv, order flow.IdentityOrder[flow.Identity]) []NodeInfoPriv {
	dup := make([]NodeInfoPriv, len(nodes))
	copy(dup, nodes)
	slices.SortFunc(dup, func(i, j NodeInfoPriv) int {
		return order(i.Identity(), j.Identity())
	})
	return dup
}

func ToIdentityList(nodes []NodeInfo) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(nodes))
	for _, node := range nodes {
		il = append(il, node.Identity())
	}
	return il
}

func PrivToIdentityList(nodes []NodeInfoPriv) flow.IdentityList {
	return ToIdentityList(PrivToNodeInfoList(nodes))
}

func PubToIdentityList(nodes []NodeInfoPub) flow.IdentityList {
	return ToIdentityList(PubToNodeInfoList(nodes))
}

func ToPubNodeInfoList(nodes []NodeInfoPriv) ([]NodeInfoPub, error) {
	pub := make([]NodeInfoPub, 0, len(nodes))
	for _, node := range nodes {
		info, err := node.Public()
		if err != nil {
			return nil, fmt.Errorf("could not read public info: %w", err)
		}
		pub = append(pub, info)
	}
	return pub, nil
}

func PrivToNodeInfoList(nodes []NodeInfoPriv) []NodeInfo {
	list := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		list = append(list, NodeInfo(node))
	}
	return list
}

func PubToNodeInfoList(nodes []NodeInfoPub) []NodeInfo {
	list := make([]NodeInfo, 0, len(nodes))
	for _, node := range nodes {
		list = append(list, NodeInfo(node))
	}
	return list
}
