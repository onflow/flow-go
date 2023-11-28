package flow

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/crypto"
)

// DefaultInitialWeight is the default initial weight for a node identity.
// It is equal to the default initial weight in the FlowIDTableStaking smart contract.
const DefaultInitialWeight = 100

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

// EpochParticipationStatus represents the status of a node's participation. Depending on what
// changes were applied to the protocol state, a node may be in one of four states:
// /   - joining - the node is not active in the current epoch and will be active in the next epoch.
// /   - active - the node was included in the EpochSetup event for the current epoch and is actively participating in the current epoch.
// /   - leaving - the node was active in the previous epoch but is not active in the current epoch.
// /   - ejected - the node has been permanently removed from the network.
//
// /            EpochSetup
// /	      ┌────────────⬤ unregistered ◯◄───────────┐
// /	┌─────▼─────┐        ┌───────────┐        ┌─────┴─────┐
// /	│  JOINING  ├───────►│  ACTIVE   ├───────►│  LEAVING  │
// /	└─────┬─────┘        └─────┬─────┘        └─────┬─────┘
// /	      │              ┌─────▼─────┐              │
// /          └─────────────►│  EJECTED  │◄─────────────┘
// /                         └───────────┘
//
// Only active nodes are allowed to perform certain tasks relative to other nodes.
// Nodes which are registered to join at the next epoch will appear in the
// identity table but aren't considered active until their first
// epoch begins. Likewise, nodes which were registered in the previous epoch
// but have left at the most recent epoch boundary will appear in the identity
// table with leaving participation status.
// A node may be ejected by either:
//   - requesting self-ejection to protect its stake in case the node operator suspects
//     the node's keys to be compromised
//   - committing a serious protocol violation or multiple smaller misdemeanours.
type EpochParticipationStatus int

const (
	EpochParticipationStatusJoining EpochParticipationStatus = iota
	EpochParticipationStatusActive
	EpochParticipationStatusLeaving
	EpochParticipationStatusEjected
)

// String returns string representation of enum value.
func (s EpochParticipationStatus) String() string {
	return [...]string{
		"EpochParticipationStatusJoining",
		"EpochParticipationStatusActive",
		"EpochParticipationStatusLeaving",
		"EpochParticipationStatusEjected",
	}[s]
}

// ParseEpochParticipationStatus converts string representation of EpochParticipationStatus into a typed value.
// An exception will be returned if failed to convert.
func ParseEpochParticipationStatus(s string) (EpochParticipationStatus, error) {
	switch s {
	case EpochParticipationStatusJoining.String():
		return EpochParticipationStatusJoining, nil
	case EpochParticipationStatusActive.String():
		return EpochParticipationStatusActive, nil
	case EpochParticipationStatusLeaving.String():
		return EpochParticipationStatusLeaving, nil
	case EpochParticipationStatusEjected.String():
		return EpochParticipationStatusEjected, nil
	default:
		return 0, fmt.Errorf("invalid epoch participation status")
	}
}

// EncodeRLP performs RLP encoding of custom type, it's need to be able to hash structures that include EpochParticipationStatus.
// No errors are expected during normal operations.
func (s EpochParticipationStatus) EncodeRLP(w io.Writer) error {
	encodable := s.String()
	err := rlp.Encode(w, encodable)
	if err != nil {
		return fmt.Errorf("could not encode rlp: %w", err)
	}
	return nil
}

// DynamicIdentity represents the dynamic part of public identity of one network participant (node).
type DynamicIdentity struct {
	EpochParticipationStatus
}

// Identity is combined from static and dynamic part and represents the full public identity of one network participant (node).
type Identity struct {
	IdentitySkeleton
	DynamicIdentity
}

// IsEjected returns true if the node is ejected from the network.
func (iy *DynamicIdentity) IsEjected() bool {
	return iy.EpochParticipationStatus == EpochParticipationStatusEjected
}

// String returns a string representation of the identity.
func (iy Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%s", iy.Role, iy.NodeID.String(), iy.Address, iy.EpochParticipationStatus.String())
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

// GetNodeID returns node ID for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetNodeID() Identifier {
	return iy.NodeID
}

// GetRole returns a node role for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetRole() Role {
	return iy.Role
}

// GetStakingPubKey returns staking public key for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetStakingPubKey() crypto.PublicKey {
	return iy.StakingPubKey
}

// GetNetworkPubKey returns network public key for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetNetworkPubKey() crypto.PublicKey {
	return iy.NetworkPubKey
}

// GetInitialWeight returns initial weight for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetInitialWeight() uint64 {
	return iy.InitialWeight
}

// GetSkeleton returns the skeleton part for the identity. It is needed to satisfy GenericIdentity constraint.
func (iy IdentitySkeleton) GetSkeleton() IdentitySkeleton {
	return iy
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
	ParticipationStatus string
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
		ParticipationStatus:       iy.EpochParticipationStatus.String(),
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
	participationStatus, err := ParseEpochParticipationStatus(ie.ParticipationStatus)
	if err != nil {
		return fmt.Errorf("could not decode epoch participation status: %w", err)
	}
	identity.EpochParticipationStatus = participationStatus
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
	return iy.EpochParticipationStatus == other.EpochParticipationStatus
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
