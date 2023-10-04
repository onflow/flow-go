package flow

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/crypto"
)

// DefaultInitialWeight is the default initial weight for a node identity.
// It is equal to the default initial weight in the FlowIDTableStaking smart contract.
const DefaultInitialWeight = 100

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|access)-([0-9a-fA-F]{64})@([\w\d]+|[\w\d][\w\d\-]*[\w\d](?:\.*[\w\d][\w\d\-]*[\w\d])*|[\w\d][\w\d\-]*[\w\d])(:[\d]+)?=(\d{1,20})$`)

// IdentitySkeleton represents the static part of a network participant's (i.e. node's) public identity.
type IdentitySkeleton struct {
	// NodeID uniquely identifies a particular node. A node's ID is fixed for
	// the duration of that node's participation in the network.
	NodeID Identifier
	// Address is the network address where the node can be reached.
	Address string
	// Role is the node's role in the network and defines its abilities and
	// responsibilities.
	Role Role
	// InitialWeight is a 'trust score' initially assigned by EpochSetup event after
	// the staking phase. The initial weights define the supermajority thresholds for
	// the cluster and security node consensus throughout the Epoch.
	InitialWeight uint64
	StakingPubKey crypto.PublicKey
	NetworkPubKey crypto.PublicKey
}

// DynamicIdentity represents the dynamic part of public identity of one network participant (node).
type DynamicIdentity struct {
	// Weight represents the node's authority to perform certain tasks relative
	// to other nodes.
	//
	// A node's weight is distinct from its stake. Stake represents the quantity
	// of FLOW tokens held by the network in escrow during the course of the node's
	// participation in the network. The stake is strictly managed by the service
	// account smart contracts.
	//
	// Nodes which are registered to join at the next epoch will appear in the
	// identity table but are considered to have zero weight up until their first
	// epoch begins. Likewise, nodes which were registered in the previous epoch
	// but have left at the most recent epoch boundary will appear in the identity
	// table with zero weight.
	Weight uint64
	// Ejected represents whether a node has been permanently removed from the
	// network. A node may be ejected by either:
	// * requesting self-ejection to protect its stake in case the node operator suspects
	//   the node's keys to be compromised
	// * committing a serious protocol violation or multiple smaller misdemeanours
	Ejected bool
}

// Identity is combined from static and dynamic part and represents the full public identity of one network participant (node).
type Identity struct {
	IdentitySkeleton
	DynamicIdentity
}

// ParseIdentity parses a string representation of an identity.
func ParseIdentity(identity string) (*Identity, error) {

	// use the regex to match the four parts of an identity
	matches := rxid.FindStringSubmatch(identity)
	if len(matches) != 6 {
		return nil, errors.New("invalid identity string format")
	}

	// none of these will error as they are checked by the regex
	var nodeID Identifier
	nodeID, err := HexStringToIdentifier(matches[2])
	if err != nil {
		return nil, err
	}
	address := matches[3] + matches[4]
	role, _ := ParseRole(matches[1])
	weight, _ := strconv.ParseUint(matches[5], 10, 64)

	// create the identity
	iy := Identity{
		IdentitySkeleton: IdentitySkeleton{
			NodeID:        nodeID,
			Address:       address,
			Role:          role,
			InitialWeight: weight,
		},
		DynamicIdentity: DynamicIdentity{
			Weight: weight,
		},
	}

	return &iy, nil
}

// String returns a string representation of the identity.
func (iy Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%d", iy.Role, iy.NodeID.String(), iy.Address, iy.Weight)
}

// String returns a string representation of the identity.
func (iy IdentitySkeleton) String() string {
	return fmt.Sprintf("%s-%s@%s", iy.Role, iy.NodeID.String(), iy.Address)
}

// ID returns a unique, persistent identifier for the identity.
// CAUTION: the ID may be chosen by a node operator, so long as it is unique.
func (iy Identity) ID() Identifier {
	return iy.NodeID
}

// Checksum returns a checksum for the identity including mutable attributes.
func (iy Identity) Checksum() Identifier {
	return MakeID(iy)
}

type encodableIdentitySkeleton struct {
	NodeID        Identifier
	Address       string `json:",omitempty"`
	Role          Role
	InitialWeight uint64
	StakingPubKey []byte
	NetworkPubKey []byte
}

type encodableIdentity struct {
	encodableIdentitySkeleton
	Weight  uint64
	Ejected bool
}

func encodableSkeletonFromIdentity(iy IdentitySkeleton) encodableIdentitySkeleton {
	ie := encodableIdentitySkeleton{
		NodeID:        iy.NodeID,
		Address:       iy.Address,
		Role:          iy.Role,
		InitialWeight: iy.InitialWeight,
	}
	if iy.StakingPubKey != nil {
		ie.StakingPubKey = iy.StakingPubKey.Encode()
	}
	if iy.NetworkPubKey != nil {
		ie.NetworkPubKey = iy.NetworkPubKey.Encode()
	}
	return ie
}

func encodableFromIdentity(iy Identity) encodableIdentity {
	return encodableIdentity{
		encodableIdentitySkeleton: encodableSkeletonFromIdentity(iy.IdentitySkeleton),
		Weight:                    iy.Weight,
		Ejected:                   iy.Ejected,
	}
}

func (iy IdentitySkeleton) MarshalJSON() ([]byte, error) {
	encodable := encodableSkeletonFromIdentity(iy)
	data, err := json.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode json: %w", err)
	}
	return data, nil
}

func (iy IdentitySkeleton) MarshalCBOR() ([]byte, error) {
	encodable := encodableSkeletonFromIdentity(iy)
	data, err := cbor.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode cbor: %w", err)
	}
	return data, nil
}

func (iy IdentitySkeleton) MarshalMsgpack() ([]byte, error) {
	encodable := encodableSkeletonFromIdentity(iy)
	data, err := msgpack.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode msgpack: %w", err)
	}
	return data, nil
}

func (iy IdentitySkeleton) EncodeRLP(w io.Writer) error {
	encodable := encodableSkeletonFromIdentity(iy)
	err := rlp.Encode(w, encodable)
	if err != nil {
		return fmt.Errorf("could not encode rlp: %w", err)
	}
	return nil
}

func (iy Identity) MarshalJSON() ([]byte, error) {
	encodable := encodableFromIdentity(iy)
	data, err := json.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode json: %w", err)
	}
	return data, nil
}

func (iy Identity) MarshalCBOR() ([]byte, error) {
	encodable := encodableFromIdentity(iy)
	data, err := cbor.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode cbor: %w", err)
	}
	return data, nil
}

func (iy Identity) MarshalMsgpack() ([]byte, error) {
	encodable := encodableFromIdentity(iy)
	data, err := msgpack.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode msgpack: %w", err)
	}
	return data, nil
}

func (iy Identity) EncodeRLP(w io.Writer) error {
	encodable := encodableFromIdentity(iy)
	err := rlp.Encode(w, encodable)
	if err != nil {
		return fmt.Errorf("could not encode rlp: %w", err)
	}
	return nil
}

func identitySkeletonFromEncodable(ie encodableIdentitySkeleton, identity *IdentitySkeleton) error {
	identity.NodeID = ie.NodeID
	identity.Address = ie.Address
	identity.Role = ie.Role
	identity.InitialWeight = ie.InitialWeight
	var err error
	if ie.StakingPubKey != nil {
		if identity.StakingPubKey, err = crypto.DecodePublicKey(crypto.BLSBLS12381, ie.StakingPubKey); err != nil {
			return fmt.Errorf("could not decode staking key: %w", err)
		}
	}
	if ie.NetworkPubKey != nil {
		if identity.NetworkPubKey, err = crypto.DecodePublicKey(crypto.ECDSAP256, ie.NetworkPubKey); err != nil {
			return fmt.Errorf("could not decode network key: %w", err)
		}
	}
	return nil
}

func identityFromEncodable(ie encodableIdentity, identity *Identity) error {
	err := identitySkeletonFromEncodable(ie.encodableIdentitySkeleton, &identity.IdentitySkeleton)
	if err != nil {
		return fmt.Errorf("could not decode identity skeleton: %w", err)
	}
	identity.Weight = ie.Weight
	identity.Ejected = ie.Ejected
	return nil
}

func (iy *IdentitySkeleton) UnmarshalJSON(b []byte) error {
	var decodable encodableIdentitySkeleton
	err := json.Unmarshal(b, &decodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identitySkeletonFromEncodable(decodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable json: %w", err)
	}
	return nil
}

func (iy *IdentitySkeleton) UnmarshalCBOR(b []byte) error {
	var encodable encodableIdentitySkeleton
	err := cbor.Unmarshal(b, &encodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identitySkeletonFromEncodable(encodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable cbor: %w", err)
	}
	return nil
}

func (iy *IdentitySkeleton) UnmarshalMsgpack(b []byte) error {
	var encodable encodableIdentitySkeleton
	err := msgpack.Unmarshal(b, &encodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identitySkeletonFromEncodable(encodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable msgpack: %w", err)
	}
	return nil
}

func (iy *Identity) UnmarshalJSON(b []byte) error {
	var decodable encodableIdentity
	err := json.Unmarshal(b, &decodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identityFromEncodable(decodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable json: %w", err)
	}
	return nil
}

func (iy *Identity) UnmarshalCBOR(b []byte) error {
	var encodable encodableIdentity
	err := cbor.Unmarshal(b, &encodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identityFromEncodable(encodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable cbor: %w", err)
	}
	return nil
}

func (iy *Identity) UnmarshalMsgpack(b []byte) error {
	var encodable encodableIdentity
	err := msgpack.Unmarshal(b, &encodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identityFromEncodable(encodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable msgpack: %w", err)
	}
	return nil
}

func (iy *IdentitySkeleton) EqualTo(other *IdentitySkeleton) bool {
	if iy.NodeID != other.NodeID {
		return false
	}
	if iy.Address != other.Address {
		return false
	}
	if iy.Role != other.Role {
		return false
	}
	if iy.InitialWeight != other.InitialWeight {
		return false
	}
	if (iy.StakingPubKey != nil && other.StakingPubKey == nil) ||
		(iy.StakingPubKey == nil && other.StakingPubKey != nil) {
		return false
	}
	if iy.StakingPubKey != nil && !iy.StakingPubKey.Equals(other.StakingPubKey) {
		return false
	}

	if (iy.NetworkPubKey != nil && other.NetworkPubKey == nil) ||
		(iy.NetworkPubKey == nil && other.NetworkPubKey != nil) {
		return false
	}
	if iy.NetworkPubKey != nil && !iy.NetworkPubKey.Equals(other.NetworkPubKey) {
		return false
	}

	return true
}

func (iy *DynamicIdentity) EqualTo(other *DynamicIdentity) bool {
	if iy.Weight != other.Weight {
		return false
	}
	if iy.Ejected != other.Ejected {
		return false
	}
	return true
}

func (iy *Identity) EqualTo(other *Identity) bool {
	if !iy.IdentitySkeleton.EqualTo(&other.IdentitySkeleton) {
		return false
	}
	if !iy.DynamicIdentity.EqualTo(&other.DynamicIdentity) {
		return false
	}
	return true
}
