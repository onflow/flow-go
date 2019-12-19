// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strconv"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/encoding"
)

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|observation)-([0-9a-fA-F]{64})@([\w\d]|[\w\d][\w\d\-]*[\w\d]\.*[\w\d]|[\w\d][\w\d\-]*[\w\d]:[\d]+)?=(\d{1,20})$`)

// Identifier represents a 32-byte unique identifier for a node.
type Identifier [32]byte

// HexStringToIdentifier converts a hex string to an identifier. The input
// must be 64 characters long and contain only valid hex characters.
func HexStringToIdentifier(hexString string) (Identifier, error) {
	var identifier Identifier
	i, err := hex.Decode(identifier[:], []byte(hexString))
	if err != nil {
		return identifier, err
	}
	if i != 32 {
		return identifier, fmt.Errorf("malformed input, expected 32 bytes (64 characters), decoded %d", i)
	}
	return identifier, nil
}

// String returns the hex string representation of the identifier.
func (id Identifier) String() string {
	return hex.EncodeToString(id[:])
}

// Identity represents a node identity.
type Identity struct {
	NodeID  Identifier
	Address string
	Role    Role
	Stake   uint64
}

// ParseIdentity parses a string representation of an identity.
func ParseIdentity(identity string) (Identity, error) {

	// use the regex to match the four parts of an identity
	matches := rxid.FindStringSubmatch(identity)
	if len(matches) != 5 {
		return Identity{}, errors.New("invalid identity string format")
	}

	// none of these will error as they are checked by the regex
	var nodeID Identifier
	nodeID, err := HexStringToIdentifier(matches[2])
	if err != nil {
		return Identity{}, err
	}
	address := matches[3]
	role, _ := ParseRole(matches[1])
	stake, _ := strconv.ParseUint(matches[4], 10, 64)

	// create the identity
	id := Identity{
		NodeID:  nodeID,
		Address: address,
		Role:    role,
		Stake:   stake,
	}

	return id, nil
}

// String returns a string representation of the identity.
func (id Identity) String() string {
	return fmt.Sprintf("%s-%x@%s=%d", id.Role, id.NodeID, id.Address, id.Stake)
}

// Encode provides a serialized version of the node identity.
func (id Identity) Encode() []byte {
	return encoding.DefaultEncoder.MustEncode(id)
}

// IdentityFilter is a filter on identities.
type IdentityFilter func(Identity) bool

// IdentityList is a list of nodes.
type IdentityList []Identity

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

// Get returns the node at the given index.
func (il IdentityList) Get(i uint) Identity {
	return il[int(i)]
}
