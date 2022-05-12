package flow

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"regexp"
	"sort"
	"strconv"

	"github.com/ethereum/go-ethereum/rlp"
	"github.com/fxamacker/cbor/v2"
	"github.com/pkg/errors"
	"github.com/vmihailenco/msgpack"

	"github.com/onflow/flow-go/crypto"
)

// DefaultInitialWeight is the default initial weight for a node identity.
const DefaultInitialWeight = 1000

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|access)-([0-9a-fA-F]{64})@([\w\d]+|[\w\d][\w\d\-]*[\w\d](?:\.*[\w\d][\w\d\-]*[\w\d])*|[\w\d][\w\d\-]*[\w\d])(:[\d]+)?=(\d{1,20})$`)

// Identity represents the public identity of one network participant (node).
type Identity struct {
	// NodeID uniquely identifies a particular node. A node's ID is fixed for
	// the duration of that node's participation in the network.
	NodeID Identifier
	// Address is the network address where the node can be reached.
	Address string
	// Role is the node's role in the network and defines its abilities and
	// responsibilities.
	Role Role
	// Weight represents the node's authority to perform certain tasks relative
	// to other nodes. For example, in the consensus committee, the node's weight
	// represents the weight assigned to its votes.
	//
	// A node's weight is distinct from its stake. Stake represents the quantity
	// of FLOW tokens held by the network in escrow during the course of the node's
	// participation in the network. The stake is strictly managed by the service
	// account smart contracts.
	//
	// Nodes which are registered to join at the next epoch will appear in the
	// identity table but are considered to have zero weight up until their first
	// epoch begins. Likewise nodes which were registered in the previous epoch
	// but have left at the most recent epoch boundary will appear in the identity
	// table with zero weight.
	Weight uint64
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
	weight, _ := strconv.ParseUint(matches[5], 10, 64)

	// create the identity
	iy := Identity{
		NodeID:  nodeID,
		Address: address,
		Role:    role,
		Weight:  weight,
	}

	return &iy, nil
}

// String returns a string representation of the identity.
func (iy Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%d", iy.Role, iy.NodeID.String(), iy.Address, iy.Weight)
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
	Address       string `json:",omitempty"`
	Role          Role
	Weight        uint64
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
	ie := encodableIdentity{iy.NodeID, iy.Address, iy.Role, iy.Weight, nil, nil}
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
	identity.Weight = ie.Weight
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

func (iy *Identity) EqualTo(other *Identity) bool {
	if iy.NodeID != other.NodeID {
		return false
	}
	if iy.Address != other.Address {
		return false
	}
	if iy.Role != other.Role {
		return false
	}
	if iy.Weight != other.Weight {
		return false
	}
	if iy.Ejected != other.Ejected {
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
	dup := make(IdentityList, 0, len(il))

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
func (il IdentityList) Selector() IdentityFilter {

	lookup := il.Lookup()
	return func(identity *Identity) bool {
		_, exists := lookup[identity.NodeID]
		return exists
	}
}

func (il IdentityList) Lookup() map[Identifier]*Identity {
	lookup := make(map[Identifier]*Identity, len(il))
	for _, identity := range il {
		lookup[identity.NodeID] = identity
	}
	return lookup
}

// Sort will sort the list using the given ordering.  This is
// not recommended for performance.  Expand the 'less' function
// in place for best performance, and don't use this function.
func (il IdentityList) Sort(less IdentityOrder) IdentityList {
	dup := il.Copy()
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i], dup[j])
	})
	return dup
}

// Sorted returns whether the list is sorted by the input ordering.
func (il IdentityList) Sorted(less IdentityOrder) bool {
	for i := 0; i < len(il)-1; i++ {
		a := il[i]
		b := il[i+1]
		if !less(a, b) {
			return false
		}
	}
	return true
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []Identifier {
	nodeIDs := make([]Identifier, 0, len(il))
	for _, id := range il {
		nodeIDs = append(nodeIDs, id.NodeID)
	}
	return nodeIDs
}

// PublicStakingKeys returns a list with the public staking keys (order preserving).
func (il IdentityList) PublicStakingKeys() []crypto.PublicKey {
	pks := make([]crypto.PublicKey, 0, len(il))
	for _, id := range il {
		pks = append(pks, id.StakingPubKey)
	}
	return pks
}

func (il IdentityList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(il)...)
}

// TotalWeight returns the total weight of all given identities.
func (il IdentityList) TotalWeight() uint64 {
	var total uint64
	for _, identity := range il {
		total += identity.Weight
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

// ByNetworkingKey gets a node from the list by network public key.
func (il IdentityList) ByNetworkingKey(key crypto.PublicKey) (*Identity, bool) {
	for _, identity := range il {
		if identity.NetworkPubKey.Equals(key) {
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

// DeterministicShuffle randomly and deterministically shuffles the identity
// list, returning the shuffled list without modifying the receiver.
func (il IdentityList) DeterministicShuffle(seed int64) IdentityList {
	dup := il.Copy()
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(il), func(i, j int) {
		dup[i], dup[j] = dup[j], dup[i]
	})
	return dup
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

// Union returns a new identity list containing every identity that occurs in
// either `il`, or `other`, or both. There are no duplicates in the output,
// where duplicates are identities with the same node ID.
// The returned IdentityList is sorted
func (il IdentityList) Union(other IdentityList) IdentityList {
	lenUnion := len(il) + len(other)
	// stores the output, the union of the two lists
	if lenUnion == 0 {
		return IdentityList{}
	}

	// add all identities together
	union := make(IdentityList, 0, lenUnion)
	union = append(union, il[:]...)
	union = append(union, other[:]...)

	// sort by node id.  This will enable duplicate checks later
	sort.Slice(union, func(p, q int) bool {
		num1 := union[p].NodeID[:]
		num2 := union[q].NodeID[:]
		lenID := len(num1)

		// assume the length is a multiple of 8, for performance.  it's 32 bytes
		for i := 0; ; i += 8 {
			chunk1 := binary.BigEndian.Uint64(num1[i:])
			chunk2 := binary.BigEndian.Uint64(num2[i:])

			if chunk1 < chunk2 {
				return true
			} else if chunk1 > chunk2 {
				return false
			} else if i >= lenID-8 {
				// we're on the last chunk of 8 bytes, the nodeid's are equal
				return false
			}
		}
	})
	// At this point, 'union' has a sorted slice of identities, potentially with duplicates.
	// We know that len(union) ≥ 1.

	// Deduplicate elements by scanning over union; we keep two index values:
	// * lastUnique: largest index of the already de-duplicated portion of the slice
	// * i: index of the element that we are inspecting whether it is a duplicate
	// Example:
	//   [▓,▓,▓,▓,▓,☐,☐,☐,☐,░ ,░,░,░,░]
	//            ↑           ↑
	//        lastUnique      i
	//   ▓ deduplicated elements in ascending order
	//   ☐ duplicated
	//   ░ elements to be inspected
	// We start with lastUnique=0 and i=1. Throughout the algorithm, we always
	// have lastUnique < i. Whenever we find that union[lastUnique] != union[i],
	// we have found the next unique element and move it at index union[lastUnique+1].
	lastUnique := 0
	for i := 1; i < lenUnion; i++ {
		if union[lastUnique].NodeID != union[i].NodeID {
			lastUnique++
			union[lastUnique] = union[i]
		}
	}
	return union[:lastUnique+1]
}

// EqualTo checks if the other list if the same, that it contains the same elements
// in the same order
func (il IdentityList) EqualTo(other IdentityList) bool {
	if len(il) != len(other) {
		return false
	}
	for i, identity := range il {
		if !identity.EqualTo(other[i]) {
			return false
		}
	}
	return true
}

// Exists takes a previously sorted Identity list and searches it for the target value
// This code is optimized, so the coding style will be different
// target:  value to search for
// CAUTION:  The identity list MUST be sorted prior to calling this method
func (il IdentityList) Exists(target *Identity) bool {
	return il.IdentifierExists(target.NodeID)
}

// IdentifierExists takes a previously sorted Identity list and searches it for the target value
// target:  value to search for
// CAUTION:  The identity list MUST be sorted prior to calling this method
func (il IdentityList) IdentifierExists(target Identifier) bool {
	left := 0
	lenList := len(il)
	if lenList == 0 {
		return false
	}

	right := lenList - 1
	mid := right >> 1
	num2 := target[:]

	// pre-calculate these 4 values for comparisons later
	var tgt [4]uint64
	tgt[0] = binary.BigEndian.Uint64(num2[:])
	tgt[1] = binary.BigEndian.Uint64(num2[8:])
	tgt[2] = binary.BigEndian.Uint64(num2[16:])
	tgt[3] = binary.BigEndian.Uint64(num2[24:])

	for {
		num1 := il[mid].NodeID[:]
		lenID := len(num1)
		i := 0

		for {
			chunk1 := binary.BigEndian.Uint64(num1[i:])
			chunk2 := tgt[i/8]

			if chunk1 < chunk2 {
				left = mid + 1
				break
			} else if chunk1 > chunk2 {
				right = mid - 1
				break
			} else if i >= lenID-8 {
				// we're on the last chunk of 8 bytes, and
				// so return true if equal -- it exists
				return true
			}

			// these 8 bytes were equal, so increment index by 8 bytes
			i += 8
		}
		if left > right {
			return false
		}
		mid = (left + right) >> 1
	}
}

// GetIndex returns the index of the identifier in the IdentityList and true
// if the identifier is found.
func (il IdentityList) GetIndex(identifier Identifier) (uint, bool) {
	index := 0
	ok := false
	for i, id := range il.NodeIDs() {
		if id == identifier {
			index = i
			ok = true
			break
		}
	}

	return uint(index), ok
}
