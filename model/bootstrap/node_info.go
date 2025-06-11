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

// NodeInfoType enumerates the two different options for
type NodeInfoType int

const (
	NodeInfoTypeInvalid NodeInfoType = iota
	NodeInfoTypePublic
	NodeInfoTypePrivate
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
	Role           flow.Role
	Address        string
	NodeID         flow.Identifier
	NetworkPrivKey encodable.NetworkPrivKey
	StakingPrivKey encodable.StakingPrivKey
}

// NodeInfoPub defines the canonical structure for encoding public node info.
type NodeInfoPub struct {
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
	Role          flow.Role
	Address       string
	NodeID        flow.Identifier
	Weight        uint64
	NetworkPubKey encodable.NetworkPubKey
	StakingPubKey encodable.StakingPubKey
	StakingPoP    encodable.StakingKeyPoP
	// Stake previously was used in place of the Weight field.
	// Deprecated: supported in decoding for backward-compatibility
	Stake uint64
}

func (info *NodeInfoPub) Equals(other *NodeInfoPub) bool {
	if other == nil {
		return false
	}
	return info.Address == other.Address &&
		info.NodeID == other.NodeID &&
		info.Role == other.Role &&
		info.Weight == other.Weight &&
		info.NetworkPubKey.PublicKey.Equals(other.NetworkPubKey.PublicKey) &&
		info.StakingPubKey.PublicKey.Equals(other.StakingPubKey.PublicKey)
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
	info.Role = decodable.Role
	info.Address = decodable.Address
	info.NodeID = decodable.NodeID
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

// NodeInfo contains information for a node. This is used during the bootstrapping
// process to represent each node. When writing node information to disk, use
// `Public` or `Private` to obtain the appropriate canonical structure.
//
// A NodeInfo instance can contain EITHER public keys OR private keys, not both.
// This can be ensured by using only using the provided constructors and NOT
// manually constructing an instance.
//
// Deprecated: There is a concern about the current usage pattern of `NodeInfo“.
// There are no build-time enforcements of using `NodeInfo` for either the private or public usage. The struct
// can mistakenly be used for both cases. Other than introducing a confusing design, developers can accidentally
// confuse the private usage as a public one, for instance by writing the private info (including the private
// keys) into a file that is publicly shared.
// There is an ongoing attempt to replace `NodeInfo` by the explicit structures `NodeInfoPriv` and `NodeInfoPub`
// in https://github.com/onflow/flow-go/pull/7476.
// It is recommended to not use `NodeInfo` in new code development in order to limit the structure usage, and to
// use `NodeInfoPriv` and `NodeInfoPub` instead.
type NodeInfo struct {

	// NodeID is the unique identifier of the node in the network
	NodeID flow.Identifier

	// Role is the flow role of the node (collection, consensus, etc...)
	Role flow.Role

	// Address is the networking address of the node (IP:PORT), not to be
	// confused with the address of the flow account associated with the node's
	// machine account.
	Address string

	// Weight is the weight of the node
	Weight uint64

	// PRIVATE Variant:
	networkPrivKey crypto.PrivateKey
	stakingPrivKey crypto.PrivateKey

	// By convention, `NodeInfo` must either include the public fields and exclude the private fields, or
	// vice versa. Mixtures are not allowed. Please check function [NodeInfoType] for the precise convention.
	//
	// PUBLIC Variant:
	networkPubKey crypto.PublicKey
	stakingPubKey crypto.PublicKey
	stakingPoP    crypto.Signature
}

func NewPublicNodeInfo(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	weight uint64,
	networkKey crypto.PublicKey,
	stakingKey crypto.PublicKey,
	stakingPoP crypto.Signature,
) NodeInfo {
	return NodeInfo{
		NodeID:        nodeID,
		Role:          role,
		Address:       addr,
		Weight:        weight,
		networkPubKey: networkKey,
		stakingPubKey: stakingKey,
		stakingPoP:    stakingPoP,
	}
}

func NewPrivateNodeInfo(
	nodeID flow.Identifier,
	role flow.Role,
	addr string,
	weight uint64,
	networkKey crypto.PrivateKey,
	stakingKey crypto.PrivateKey,
) (NodeInfo, error) {
	pop, err := crypto.BLSGeneratePOP(stakingKey)
	if err != nil {
		return NodeInfo{}, fmt.Errorf("failed to generate PoP: %w", err)
	}

	return NodeInfo{
		NodeID:         nodeID,
		Role:           role,
		Address:        addr,
		Weight:         weight,
		networkPrivKey: networkKey,
		stakingPrivKey: stakingKey,
		networkPubKey:  networkKey.PublicKey(),
		stakingPubKey:  stakingKey.PublicKey(),
		stakingPoP:     pop,
	}, nil
}

// Type returns the type of the node info instance.
func (node NodeInfo) Type() NodeInfoType {
	if node.networkPrivKey != nil && node.stakingPrivKey != nil {
		return NodeInfoTypePrivate
	}
	if node.networkPubKey != nil && node.stakingPubKey != nil && node.stakingPoP != nil {
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

func (node NodeInfo) StakingPoP() (crypto.Signature, error) {
	if node.stakingPoP != nil {
		return node.stakingPoP, nil
	}
	pop, err := crypto.BLSGeneratePOP(node.stakingPrivKey)
	if err != nil {
		return nil, fmt.Errorf("staking PoP generation failed: %w", err)
	}
	return pop, nil
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
		NetworkPrivKey: encodable.NetworkPrivKey{PrivateKey: node.networkPrivKey},
		StakingPrivKey: encodable.StakingPrivKey{PrivateKey: node.stakingPrivKey},
	}, nil
}

// Public returns the canonical encodable structure holding the node's public information.
// It derives the networking and staking public keys, as well as the Proof of Possession (PoP) of the staking private key
// if they are not already provided in the NodeInfo.
//
// It errors, if there is a problem generating the staking key PoP.
func (node NodeInfo) Public() (NodeInfoPub, error) {
	stakingPoP, err := node.StakingPoP()
	if err != nil {
		return NodeInfoPub{}, fmt.Errorf("failed to generate staking PoP: %w", err)
	}

	return NodeInfoPub{
		Role:          node.Role,
		Address:       node.Address,
		NodeID:        node.NodeID,
		Weight:        node.Weight,
		NetworkPubKey: encodable.NetworkPubKey{PublicKey: node.NetworkPubKey()},
		StakingPubKey: encodable.StakingPubKey{PublicKey: node.StakingPubKey()},
		StakingPoP:    encodable.StakingKeyPoP{Signature: stakingPoP},
	}, nil
}

// PartnerPublic returns the public data for a partner node.
func (node NodeInfo) PartnerPublic() (PartnerNodeInfoPub, error) {

	stakingPoP, err := node.StakingPoP()
	if err != nil {
		return PartnerNodeInfoPub{}, fmt.Errorf("failed to generate staking PoP: %w", err)
	}
	return PartnerNodeInfoPub{
		Role:          node.Role,
		Address:       node.Address,
		NodeID:        node.NodeID,
		NetworkPubKey: encodable.NetworkPubKey{PublicKey: node.NetworkPubKey()},
		StakingPubKey: encodable.StakingPubKey{PublicKey: node.StakingPubKey()},
		StakingPoP:    stakingPoP,
	}, nil
}

// Identity returns the node info as a public Flow identity.
func (node NodeInfo) Identity() *flow.Identity {
	identity := &flow.Identity{
		IdentitySkeleton: flow.IdentitySkeleton{
			NodeID:        node.NodeID,
			Address:       node.Address,
			Role:          node.Role,
			InitialWeight: node.Weight,
			StakingPubKey: node.stakingPubKey,
			NetworkPubKey: node.networkPubKey,
		},
		DynamicIdentity: flow.DynamicIdentity{
			EpochParticipationStatus: flow.EpochParticipationStatusActive,
		},
	}
	return identity
}

// PrivateNodeInfoFromIdentity builds a NodeInfo from a flow Identity.
// WARNING: Nothing enforces that the output NodeInfo's keys are corresponding to the input Identity.
func PrivateNodeInfoFromIdentity(identity *flow.Identity, networkKey, stakingKey crypto.PrivateKey) (NodeInfo, error) {
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
		if node.Role != role {
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

func ToIdentityList(nodes []NodeInfo) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(nodes))
	for _, node := range nodes {
		il = append(il, node.Identity())
	}
	return il
}

func ToPublicNodeInfoList(nodes []NodeInfo) ([]NodeInfoPub, error) {
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
