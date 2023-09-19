package flow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/utils/rand"
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

type encodableIdentity struct {
	NodeID        Identifier
	Address       string `json:",omitempty"`
	Role          Role
	InitialWeight uint64
	Weight        uint64
	Ejected       bool
	StakingPubKey []byte
	NetworkPubKey []byte
}

// decodableIdentity provides backward-compatible decoding of old models
// which use the Stake field in place of Weight.
type decodableIdentity struct {
	encodableIdentity
	// Stake previously was used in place of the Weight field.
	// Deprecated: supported in decoding for backward-compatibility
	Stake uint64
}

func encodableFromIdentity(iy Identity) (encodableIdentity, error) {
	ie := encodableIdentity{
		NodeID:        iy.NodeID,
		Address:       iy.Address,
		Role:          iy.Role,
		InitialWeight: iy.InitialWeight,
		Weight:        iy.Weight,
		Ejected:       iy.Ejected,
	}
	if iy.StakingPubKey != nil {
		ie.StakingPubKey = iy.StakingPubKey.Encode()
	}
	if iy.NetworkPubKey != nil {
		ie.NetworkPubKey = iy.NetworkPubKey.Encode()
	}
	return ie, nil
}

func (iy Identity) MarshalJSON() ([]byte, error) {
	encodable, err := encodableFromIdentity(iy)
	if err != nil {
		return nil, fmt.Errorf("could not convert identity to encodable: %w", err)
	}

	data, err := json.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode json: %w", err)
	}
	return data, nil
}

func (iy Identity) MarshalCBOR() ([]byte, error) {
	encodable, err := encodableFromIdentity(iy)
	if err != nil {
		return nil, fmt.Errorf("could not convert identity to encodable: %w", err)
	}
	data, err := cbor.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode cbor: %w", err)
	}
	return data, nil
}

func (iy Identity) MarshalMsgpack() ([]byte, error) {
	encodable, err := encodableFromIdentity(iy)
	if err != nil {
		return nil, fmt.Errorf("could not convert to encodable: %w", err)
	}
	data, err := msgpack.Marshal(encodable)
	if err != nil {
		return nil, fmt.Errorf("could not encode msgpack: %w", err)
	}
	return data, nil
}

func (iy Identity) EncodeRLP(w io.Writer) error {
	encodable, err := encodableFromIdentity(iy)
	if err != nil {
		return fmt.Errorf("could not convert to encodable: %w", err)
	}
	err = rlp.Encode(w, encodable)
	if err != nil {
		return fmt.Errorf("could not encode rlp: %w", err)
	}
	return nil
}

func identityFromEncodable(ie encodableIdentity, identity *Identity) error {
	identity.NodeID = ie.NodeID
	identity.Address = ie.Address
	identity.Role = ie.Role
	identity.InitialWeight = ie.InitialWeight
	identity.Weight = ie.Weight
	identity.Ejected = ie.Ejected
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

func (iy *Identity) UnmarshalJSON(b []byte) error {
	var decodable decodableIdentity
	err := json.Unmarshal(b, &decodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	// compat: translate Stake fields to Weight
	if decodable.Stake != 0 {
		if decodable.Weight != 0 {
			return fmt.Errorf("invalid identity with both Stake and Weight fields")
		}
		decodable.Weight = decodable.Stake
	}
	err = identityFromEncodable(decodable.encodableIdentity, iy)
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

// GenericIdentity defines a constraint for generic identities.
// Golang doesn't support constraint with fields(for time being) so we have to define this interface
// with getter methods.
// Details here: https://github.com/golang/go/issues/51259.
type GenericIdentity interface {
	Identity | IdentitySkeleton
	GetNodeID() Identifier
	GetRole() Role
	GetStakingPubKey() crypto.PublicKey
	GetNetworkPubKey() crypto.PublicKey
	GetInitialWeight() uint64
	GetSkeleton() IdentitySkeleton
}

func (iy IdentitySkeleton) GetNodeID() Identifier {
	return iy.NodeID
}

func (iy IdentitySkeleton) GetRole() Role {
	return iy.Role
}

func (iy IdentitySkeleton) GetStakingPubKey() crypto.PublicKey {
	return iy.StakingPubKey
}

func (iy IdentitySkeleton) GetNetworkPubKey() crypto.PublicKey {
	return iy.NetworkPubKey
}

func (iy IdentitySkeleton) GetInitialWeight() uint64 {
	return iy.InitialWeight
}

func (iy IdentitySkeleton) GetSkeleton() IdentitySkeleton {
	return iy
}

// IdentityFilter is a filter on identities.
type IdentityFilter[T GenericIdentity] func(*T) bool

// IdentityOrder is a sort for identities.
type IdentityOrder[T GenericIdentity] func(*T, *T) bool

// IdentityMapFunc is a modifier function for map operations for identities.
// Identities are COPIED from the source slice.
type IdentityMapFunc[T GenericIdentity] func(T) T

// IdentitySkeletonList is a list of nodes skeletons. We use a type alias instead of defining a new type
// since go generics doesn't support implicit conversion between types.
type IdentitySkeletonList = GenericIdentityList[IdentitySkeleton]

// IdentityList is a list of nodes. We use a type alias instead of defining a new type
// since go generics doesn't support implicit conversion between types.
type IdentityList = GenericIdentityList[Identity]

type GenericIdentityList[T GenericIdentity] []*T

// Filter will apply a filter to the identity list.
func (il GenericIdentityList[T]) Filter(filter IdentityFilter[T]) GenericIdentityList[T] {
	var dup GenericIdentityList[T]
IDLoop:
	for _, identity := range il {
		if !filter(identity) {
			continue IDLoop
		}
		dup = append(dup, identity)
	}
	return dup
}

// Map returns a new identity list with the map function f applied to a copy of
// each identity.
//
// CAUTION: this relies on structure copy semantics. Map functions that modify
// an object referenced by the input Identity structure will modify identities
// in the source slice as well.
func (il GenericIdentityList[T]) Map(f IdentityMapFunc[T]) GenericIdentityList[T] {
	dup := make(GenericIdentityList[T], 0, len(il))
	for _, identity := range il {
		next := f(*identity)
		dup = append(dup, &next)
	}
	return dup
}

// Copy returns a copy of IdentityList. The resulting slice uses a different
// backing array, meaning appends and insert operations on either slice are
// guaranteed to only affect that slice.
//
// Copy should be used when modifying an existing identity list by either
// appending new elements, re-ordering, or inserting new elements in an
// existing index.
//
// CAUTION:
// All Identity fields are deep-copied, _except_ for their keys, which
// are copied by reference.
func (il GenericIdentityList[T]) Copy() GenericIdentityList[T] {
	dup := make(GenericIdentityList[T], 0, len(il))

	lenList := len(il)

	// performance tests show this is faster than 'range'
	for i := 0; i < lenList; i++ {
		// copy the object
		next := *(il[i])
		dup = append(dup, &next)
	}
	return dup
}

// Selector returns an identity filter function that selects only identities
// within this identity list.
func (il GenericIdentityList[T]) Selector() IdentityFilter[T] {
	lookup := il.Lookup()
	return func(identity *T) bool {
		_, exists := lookup[(*identity).GetNodeID()]
		return exists
	}
}

func (il GenericIdentityList[T]) Lookup() map[Identifier]*T {
	lookup := make(map[Identifier]*T, len(il))
	for _, identity := range il {
		lookup[(*identity).GetNodeID()] = identity
	}
	return lookup
}

// Sort will sort the list using the given ordering.  This is
// not recommended for performance.  Expand the 'less' function
// in place for best performance, and don't use this function.
func (il GenericIdentityList[T]) Sort(less IdentityOrder[T]) GenericIdentityList[T] {
	dup := il.Copy()
	slices.SortFunc(dup, less)
	return dup
}

// Sorted returns whether the list is sorted by the input ordering.
func (il GenericIdentityList[T]) Sorted(less IdentityOrder[T]) bool {
	return slices.IsSortedFunc(il, less)
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il GenericIdentityList[T]) NodeIDs() IdentifierList {
	nodeIDs := make([]Identifier, 0, len(il))
	for _, id := range il {
		nodeIDs = append(nodeIDs, (*id).GetNodeID())
	}
	return nodeIDs
}

// PublicStakingKeys returns a list with the public staking keys (order preserving).
func (il GenericIdentityList[T]) PublicStakingKeys() []crypto.PublicKey {
	pks := make([]crypto.PublicKey, 0, len(il))
	for _, id := range il {
		pks = append(pks, (*id).GetStakingPubKey())
	}
	return pks
}

// ID uniquely identifies a list of identities, by node ID. This can be used
// to perpetually identify a group of nodes, even if mutable fields of some nodes
// are changed, as node IDs are immutable.
// CAUTION:
//   - An IdentityList's ID is a cryptographic commitment to only node IDs. A node operator
//     can freely choose the ID for their node. There is no relationship whatsoever between
//     a node's ID and keys.
//   - To generate a cryptographic commitment for the full IdentityList, use method `Checksum()`.
//   - The outputs of `IdentityList.ID()` and `IdentityList.Checksum()` are both order-sensitive.
//     Therefore, the `IdentityList` must be in canonical order, unless explicitly specified
//     otherwise by the protocol.
func (il GenericIdentityList[T]) ID() Identifier {
	return il.NodeIDs().ID()
}

// Checksum generates a cryptographic commitment to the full IdentityList, including mutable fields.
// The checksum for the same group of identities (by NodeID) may change from block to block.
func (il GenericIdentityList[T]) Checksum() Identifier {
	return MakeID(il)
}

// TotalWeight returns the total weight of all given identities.
func (il GenericIdentityList[T]) TotalWeight() uint64 {
	var total uint64
	for _, identity := range il {
		total += (*identity).GetInitialWeight()
	}
	return total
}

// Count returns the count of identities.
func (il GenericIdentityList[T]) Count() uint {
	return uint(len(il))
}

// ByIndex returns the node at the given index.
func (il GenericIdentityList[T]) ByIndex(index uint) (*T, bool) {
	if index >= uint(len(il)) {
		return nil, false
	}
	return il[int(index)], true
}

// ByNodeID gets a node from the list by node ID.
func (il GenericIdentityList[T]) ByNodeID(nodeID Identifier) (*T, bool) {
	for _, identity := range il {
		if (*identity).GetNodeID() == nodeID {
			return identity, true
		}
	}
	return nil, false
}

// ByNetworkingKey gets a node from the list by network public key.
func (il GenericIdentityList[T]) ByNetworkingKey(key crypto.PublicKey) (*T, bool) {
	for _, identity := range il {
		if (*identity).GetNetworkPubKey().Equals(key) {
			return identity, true
		}
	}
	return nil, false
}

// Sample returns non-deterministic random sample from the `IdentityList`
func (il GenericIdentityList[T]) Sample(size uint) (GenericIdentityList[T], error) {
	n := uint(len(il))
	dup := make(GenericIdentityList[T], 0, n)
	dup = append(dup, il...)
	if n < size {
		size = n
	}
	swap := func(i, j uint) {
		dup[i], dup[j] = dup[j], dup[i]
	}
	err := rand.Samples(n, size, swap)
	if err != nil {
		return nil, fmt.Errorf("failed to sample identity list: %w", err)
	}
	return dup[:size], nil
}

// Shuffle randomly shuffles the identity list (non-deterministic),
// and returns the shuffled list without modifying the receiver.
func (il GenericIdentityList[T]) Shuffle() (GenericIdentityList[T], error) {
	return il.Sample(uint(len(il)))
}

// SamplePct returns a random sample from the receiver identity list. The
// sample contains `pct` percentage of the list. The sample is rounded up
// if `pct>0`, so this will always select at least one identity.
//
// NOTE: The input must be between 0-1.
func (il GenericIdentityList[T]) SamplePct(pct float64) (GenericIdentityList[T], error) {
	if pct <= 0 {
		return GenericIdentityList[T]{}, nil
	}

	count := float64(il.Count()) * pct
	size := uint(math.Round(count))
	// ensure we always select at least 1, for non-zero input
	if size == 0 {
		size = 1
	}

	return il.Sample(size)
}

// Union returns a new identity list containing every identity that occurs in
// either `il`, or `other`, or both. There are no duplicates in the output,
// where duplicates are identities with the same node ID.
// Receiver `il` and/or method input `other` can be nil or empty.
// The returned IdentityList is sorted in canonical order.
func (il GenericIdentityList[T]) Union(other GenericIdentityList[T]) GenericIdentityList[T] {
	maxLen := len(il) + len(other)

	union := make(GenericIdentityList[T], 0, maxLen)
	set := make(map[Identifier]struct{}, maxLen)

	for _, list := range []GenericIdentityList[T]{il, other} {
		for _, id := range list {
			if _, isDuplicate := set[(*id).GetNodeID()]; !isDuplicate {
				set[(*id).GetNodeID()] = struct{}{}
				union = append(union, id)
			}
		}
	}

	slices.SortFunc(union, func(a, b *T) bool {
		lhs, rhs := (*a).GetNodeID(), (*b).GetNodeID()
		return bytes.Compare(lhs[:], rhs[:]) < 0
	})

	return union
}

// EqualTo checks if the other list if the same, that it contains the same elements
// in the same order
func (il GenericIdentityList[T]) EqualTo(other GenericIdentityList[T]) bool {
	// TODO: temporary
	//return slices.EqualFunc(il, other, func(a, b *T) bool {
	//	return (*a)..EqualTo(b)
	//})
	return il.ID() == other.ID()
}

// Exists takes a previously sorted Identity list and searches it for the target value
// This code is optimized, so the coding style will be different
// target:  value to search for
// CAUTION:  The identity list MUST be sorted prior to calling this method
func (il GenericIdentityList[T]) Exists(target *T) bool {
	return il.IdentifierExists((*target).GetNodeID())
}

// IdentifierExists takes a previously sorted Identity list and searches it for the target value
// target:  value to search for
// CAUTION:  The identity list MUST be sorted prior to calling this method
func (il GenericIdentityList[T]) IdentifierExists(target Identifier) bool {
	_, ok := slices.BinarySearchFunc(il, target, func(a *T, b Identifier) int {
		lhs := (*a).GetNodeID()
		return bytes.Compare(lhs[:], b[:])
	})
	return ok
}

// GetIndex returns the index of the identifier in the IdentityList and true
// if the identifier is found.
func (il GenericIdentityList[T]) GetIndex(target Identifier) (uint, bool) {
	i := slices.IndexFunc(il, func(a *T) bool {
		return (*a).GetNodeID() == target
	})
	if i == -1 {
		return 0, false
	}
	return uint(i), true
}

// ToSkeleton converts the identity list to a list of identity skeletons.
func (il GenericIdentityList[T]) ToSkeleton() IdentitySkeletonList {
	skeletons := make(IdentitySkeletonList, len(il))
	for i, id := range il {
		v := (*id).GetSkeleton()
		skeletons[i] = &v
	}
	return skeletons
}
