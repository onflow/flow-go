package flow

import (
	"encoding/hex"
	"fmt"
)

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

// Entity defines how flow entities should be defined
// Entities are flat data structures holding multiple data fields.
// Entities don't includes nested entities, they only include pointers to
// other entities. for example they keep an slice of entity commits instead
// of keeping an slice of entity object itself. This simplifies storage, signature and validation
// of entities.
type Entity interface {

	// ID returns a unique id for this entity using a hash of the immutable
	// fields of the entity.
	ID() Identifier

	// Checksum returns a unique checksum for the entity, including the mutable
	// data such as signatures.
	Checksum() Identifier
}

// Proof contains proof that an entity is part of a EntityList
type Proof []byte

// EntityList is a list of entities of the same type
type EntityList interface {
	EntitySet

	// HasIndex checks if the list has an entity at the given index.
	HasIndex(i uint) bool

	// ByIndex returns an entity from the list by index
	ByIndex(i uint) (Entity, bool)

	// ByIndexWithProof returns an entity from the list by index and proof of membership
	ByIndexWithProof(i uint) (Entity, Proof, bool)
}

// EntitySet holds a set of entities (order doesn't matter)
type EntitySet interface {

	// Insert adds an entity to the data structure.
	Insert(Entity) bool

	// Remove removes an entity from the data structure.
	Remove(Entity) bool

	// Items returns all items of the collection.
	Items() []Entity

	// Size returns the number of entities in the data structure.
	Size() uint

	// Fingerprint returns a unique identifier for all entities of the data
	// structure.
	Fingerprint() Identifier

	// ByID returns the entity with the given fingerprint.
	ByID(id Identifier) (Entity, bool)

	// if the set has an specific member providing proof of membership
	ByIDWithProof(id Identifier) (bool, Proof, error)
}
