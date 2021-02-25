package flow

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/crypto"
)

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|access)-([0-9a-fA-F]{64})@([\w\d]+|[\w\d][\w\d\-]*[\w\d](?:\.*[\w\d][\w\d\-]*[\w\d])*|[\w\d][\w\d\-]*[\w\d])(:[\d]+)?=(\d{1,20})$`)

// Identity represents a node identity.
type Identity struct {
	// NodeID uniquely identifies a particular node. A node's ID is fixed for
	// the duration of that node's participation in the network.
	NodeID  Identifier
	Address string
	Role    Role
	// Stake represents the node's *weight*. The stake (quantity of $FLOW held
	// in escrow during the node's participation) is strictly managed by the
	// service account. The protocol software strictly considers weight, which
	// represents how much voting power a given node has.
	//
	// NOTE: Nodes that are registered for an upcoming epoch, or that are in
	// the process of un-staking, have 0 weight.
	//
	// TODO: to be renamed to Weight
	Stake uint64
	// Ejected represents whether a node has been permanently removed from the
	// network. A node may be ejected for either:
	// * committing one protocol felony
	// * committing a series of protocol misdemeanours
	Ejected       bool
	StakingPubKey crypto.PublicKey
	NetworkPubKey crypto.PublicKey
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
	stake, _ := strconv.ParseUint(matches[5], 10, 64)

	// create the identity
	iy := Identity{
		NodeID:  nodeID,
		Address: address,
		Role:    role,
		Stake:   stake,
	}

	return &iy, nil
}

// String returns a string representation of the identity.
func (iy Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%d", iy.Role, iy.NodeID.String(), iy.Address, iy.Stake)
}

// ID returns a unique identifier for the identity.
func (iy Identity) ID() Identifier {
	return iy.NodeID
}

// Checksum returns a checksum for the identity including mutable attributes.
func (iy Identity) Checksum() Identifier {
	return MakeID(iy)
}

type encodableIdentity struct {
	NodeID        Identifier
	Address       string
	Role          Role
	Stake         uint64
	StakingPubKey []byte
	NetworkPubKey []byte
}

func encodableFromIdentity(iy Identity) (encodableIdentity, error) {
	ie := encodableIdentity{iy.NodeID, iy.Address, iy.Role, iy.Stake, nil, nil}
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

func identityFromEncodable(ie encodableIdentity, identity *Identity) error {
	identity.NodeID = ie.NodeID
	identity.Address = ie.Address
	identity.Role = ie.Role
	identity.Stake = ie.Stake
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
	var encodable encodableIdentity
	err := json.Unmarshal(b, &encodable)
	if err != nil {
		return fmt.Errorf("could not decode json: %w", err)
	}
	err = identityFromEncodable(encodable, iy)
	if err != nil {
		return fmt.Errorf("could not convert from encodable json: %w", err)
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

// IdentityFilter is a filter on identities.
type IdentityFilter func(*Identity) bool

// IdentityOrder is a sort for identities.
type IdentityOrder func(*Identity, *Identity) bool

// IdentityMapFunc is a modifier function for map operations for identities.
// Identities are COPIED from the source slice.
type IdentityMapFunc func(Identity) Identity

// IdentityList is a list of nodes.
type IdentityList []*Identity

// Filter will apply a filter to the identity list.
func (il IdentityList) Filter(filter IdentityFilter) IdentityList {
	var dup IdentityList
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
func (il IdentityList) Map(f IdentityMapFunc) IdentityList {
	dup := make(IdentityList, 0, len(il))
	for _, identity := range il {
		next := f(*identity)
		dup = append(dup, &next)
	}
	return dup
}

// Copy returns a copy of the receiver. The resulting slice uses a different
// backing array, meaning appends and insert operations on either slice are
// guaranteed to only affect that slice.
//
// Copy should be used when modifying an existing identity list by either
// appending new elements, re-ordering, or inserting new elements in an
// existing index.
func (il IdentityList) Copy() IdentityList {
	dup := make(IdentityList, len(il))
	copy(dup, il)
	return dup
}

// Selector returns an identity filter function that selects only identities
// within this identity list.
func (il IdentityList) Selector() IdentityFilter {

	lookup := il.Lookup()
	return func(identity *Identity) bool {
		_, exists := lookup[identity.NodeID]
		return exists
	}
}

func (il IdentityList) Lookup() map[Identifier]struct{} {
	lookup := make(map[Identifier]struct{})
	for _, identity := range il {
		lookup[identity.NodeID] = struct{}{}
	}
	return lookup
}

// Order will sort the list using the given sort function.
func (il IdentityList) Order(less IdentityOrder) IdentityList {
	dup := il.Copy()
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i], dup[j])
	})
	return dup
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []Identifier {
	nodeIDs := make([]Identifier, 0, len(il))
	for _, id := range il {
		nodeIDs = append(nodeIDs, id.NodeID)
	}
	return nodeIDs
}

func (il IdentityList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(il)...)
}

// TotalStake returns the total stake of all given identities.
func (il IdentityList) TotalStake() uint64 {
	var total uint64
	for _, identity := range il {
		total += identity.Stake
	}
	return total
}

// Count returns the count of identities.
func (il IdentityList) Count() uint {
	return uint(len(il))
}

// ByIndex returns the node at the given index.
func (il IdentityList) ByIndex(index uint) (*Identity, bool) {
	if index >= uint(len(il)) {
		return nil, false
	}
	return il[int(index)], true
}

// ByNodeID gets a node from the list by node ID.
func (il IdentityList) ByNodeID(nodeID Identifier) (*Identity, bool) {
	for _, identity := range il {
		if identity.NodeID == nodeID {
			return identity, true
		}
	}
	return nil, false
}

// Sample returns simple random sample from the `IdentityList`
func (il IdentityList) Sample(size uint) IdentityList {
	n := uint(len(il))
	if size > n {
		size = n
	}
	dup := make([]*Identity, 0, n)
	dup = append(dup, il...)
	for i := uint(0); i < size; i++ {
		j := uint(rand.Intn(int(n - i)))
		dup[i], dup[j+i] = dup[j+i], dup[i]
	}
	return dup[:size]
}

// DeterministicSample returns deterministic random sample from the `IdentityList` using the given seed
func (il IdentityList) DeterministicSample(size uint, seed int64) IdentityList {
	rand.Seed(seed)
	return il.Sample(size)
}

// SamplePct returns a random sample from the receiver identity list. The
// sample contains `pct` percentage of the list. The sample is rounded up
// if `pct>0`, so this will always select at least one identity.
//
// NOTE: The input must be between 0-1.
func (il IdentityList) SamplePct(pct float64) IdentityList {
	if pct <= 0 {
		return IdentityList{}
	}

	count := float64(il.Count()) * pct
	size := uint(math.Round(count))
	// ensure we always select at least 1, for non-zero input
	if size == 0 {
		size = 1
	}

	return il.Sample(size)
}

// StakingKeys returns a list of the staking public keys for the identities.
func (il IdentityList) StakingKeys() []crypto.PublicKey {
	keys := make([]crypto.PublicKey, 0, len(il))
	for _, identity := range il {
		keys = append(keys, identity.StakingPubKey)
	}
	return keys
}

// Union returns a new identity list containing every identity that occurs in
// either `il`, or `other`, or both. There are no duplicates in the output,
// where duplicates are identities with the same node ID.
func (il IdentityList) Union(other IdentityList) IdentityList {

	// stores the output, the union of the two lists
	union := make(IdentityList, 0, len(il)+len(other))
	// efficient lookup to avoid duplicates
	lookup := make(map[Identifier]struct{})

	// add all identities, omitted duplicates
	for _, identity := range append(il.Copy(), other...) {
		if _, exists := lookup[identity.NodeID]; exists {
			continue
		}
		union = append(union, identity)
		lookup[identity.NodeID] = struct{}{}
	}

	return union
}
