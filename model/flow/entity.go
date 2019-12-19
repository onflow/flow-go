package flow

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/crypto"
)

// Identifier represents a 32-byte unique identifier for a node.
type Identifier [32]byte

// Fingerprint is a hash of an entity to verify the content
// TODO update this to be a fixed size array
type Fingerprint crypto.Hash

// Hex returns the hex string representation of the fingerprint.
func (fp Fingerprint) Hex() string {
	return hex.EncodeToString(fp)
}

// Entity defines how flow entities should be defined
// Entities are flat data structures holding multiple data fields.
// Entities don't includes nested entities, they only include pointers to
// other entities. for example they keep an slice of entity commits instead
// of keeping an slice of entity object itself. This simplifies storage, signature and validation
// of entities.
type Entity interface {
	// ID returns a unique id for this entity,
	// Note that ID is built based on the immutable and rudimentary
	// part of the content, it doesn't hold the state of the entity.
	ID() Identifier
	// Body returns the body party of the entity; critical properties
	// that requires validation upon receiving. In other words, properties
	// outside of the body is controlled locally and is not trustable by other nodes.
	Body() interface{}
	// Fingerprint returns a proof (some sort of root hash) for content of the body.
	Fingerprint() Fingerprint
}

// MembershipProof contains proof that an entity is part of a EntityList
type MembershipProof []byte

// EntityList is a list of entities of the same type
type EntityList interface {
	// Append an entity to the list
	Append(e Entity) error
	// Items return all entities
	Items() []Entity
	// Fingerprint returns a proof (some sort of root hash) on the content in the list
	Fingerprint() Fingerprint
	// ByFingerprint returns an entity from the list by entity commit
	ByFingerprint(f Fingerprint) Entity
	// ByIndex returns an entity from the list by index
	ByIndex(i uint64) Entity
	// ByIndexWithProof returns an entity from the list by index and proof of membership
	ByIndexWithProof(i uint64) (Entity, MembershipProof)
}

// EntitySet holds a set of entities (order doesn't matter)
type EntitySet interface {
	// Add an entity to the set
	Add(e Entity) error
	// Fingerprint returns some sort of root hash of the items in the set
	Fingerprint() Fingerprint
	// if the set has an specific member
	Has(entityFingerprint Fingerprint) (bool, error)
	// if the set has an specific member providing proof of membership
	HasWithProof() (bool, MembershipProof, error)
}
