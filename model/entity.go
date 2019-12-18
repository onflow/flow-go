package model

import "github.com/dapperlabs/flow-go/crypto"

// Identifier represents a 32-byte unique identifier for a node.
type Identifier [32]byte

// Commit is a hash of an entity to verify the content
type Commit crypto.Hash

// Entity defines how flow entities should be defined
// Entities are flat data structures holding multiple data fields.
// Entities don't includes nested entities, they only include pointers to
// other entities. for example they keep an slice of entity commits instead
// of keeping an slice of entity object itself. This simplifies storage, signature and validation
// of entities.
type Entity interface {
	// returns a unique id for this entity,
	// note that ID is built based on the immutable and rudimentary
	// part of the content, it doesn't holds the state of the entity.
	ID() Identifier
	// Body returns the body party of the entity; critical properties
	// that requires validation upon receiving. In other words, properties
	// outside of the body is controlled locally and is not trustable by other nodes.
	Body() interface{}
	// Commit returns a proof (some sort of root hash) for content of the body.
	Commit() Commit
}

// MembershipProof contains proof that an entity is part of a EntityList
type MembershipProof []byte

// EntityList is a list of entities of the same type
type EntityList interface {
	// Append an entity to the list
	AppendItem(e Entity) error
	// Items return all entities
	Items() []Entity
	// Commit returns a proof (some sort of root hash) on the content in the list
	Commit() Commit
	// ByCommit returns an entity from the list by entity commit
	ByCommit(c Commit) Entity
	// ByCommit returns an entity from the list by index
	ByIndex(i uint64) Entity
	// ByCommit returns an entity from the list by index and proof of membership
	ByIndexWithProof(i uint64) (Entity, MembershipProof)
}

// EntitySet holds a set of entities (order doesn't matter)
type EntitySet interface {
	// Add an entity to the set
	AddItem(e Entity) error
	// Commit returns a proof (some sort of root hash) on the content in the set
	Commit() Commit
	// if the set has an specific member
	Has(entityCommit Commit) (bool, error)
	// if the set has an specific member providing proof of membership
	HasWithProof() (bool, MembershipProof, error)
}
