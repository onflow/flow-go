package flow

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
)

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|observation)-([0-9a-fA-F]{64})@([\w\d]+|[\w\d][\w\d\-]*[\w\d](?:\.*[\w\d][\w\d\-]*[\w\d])*|[\w\d][\w\d\-]*[\w\d])(:[\d]+)?=(\d{1,20})$`)

// Identity represents a node identity.
type Identity struct {
	NodeID             Identifier
	Address            string
	Role               Role
	Stake              uint64
	StakingPubKey      crypto.PublicKey
	RandomBeaconPubKey crypto.PublicKey
	NetworkPubKey      crypto.PublicKey
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
	id := Identity{
		NodeID:  nodeID,
		Address: address,
		Role:    role,
		Stake:   stake,
	}

	return &id, nil
}

// String returns a string representation of the identity.
func (id Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%d", id.Role, id.NodeID.String(), id.Address, id.Stake)
}

// ID returns a unique identifier for the identity.
func (id Identity) ID() Identifier {
	return id.NodeID
}

// Checksum returns a checksum for the identity including mutable attributes.
func (id Identity) Checksum() Identifier {
	return MakeID(id)
}

// IdentityFilter is a filter on identities.
type IdentityFilter func(*Identity) bool

// IdentityList is a list of nodes.
type IdentityList []*Identity

// Filter will apply a filter to the identity list.
func (il IdentityList) Filter(filters ...IdentityFilter) IdentityList {
	var dup IdentityList
IDLoop:
	for _, id := range il {
		for _, filter := range filters {
			if !filter(id) {
				continue IDLoop
			}
		}
		dup = append(dup, id)
	}
	return dup
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []Identifier {
	ids := make([]Identifier, 0, len(il))
	for _, id := range il {
		ids = append(ids, id.NodeID)
	}
	return ids
}

func (il IdentityList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(il)...)
}

// TotalStake returns the total stake of all given identities.
func (il IdentityList) TotalStake() uint64 {
	var total uint64
	for _, id := range il {
		total += id.Stake
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
	if size > uint(len(il)) {
		size = uint(len(il))
	}
	dup := make([]*Identity, 0, len(il))
	for _, identity := range il {
		dup = append(dup, identity)
	}
	rand.Shuffle(len(dup), func(i int, j int) {
		dup[i], dup[j] = dup[j], dup[i]
	})
	return dup[:size]
}
