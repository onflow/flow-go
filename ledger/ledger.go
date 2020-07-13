package ledger

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/dapperlabs/flow-go/module"
)

// Ledger takes care of storing and reading registers (key, value pairs)
type Ledger interface {
	module.ReadyDoneAware

	EmptyStateCommitment() StateCommitment

	// Get returns values for the given slice of keys at specific state commitment
	Get(query *Query) (values []Value, err error)

	// Update updates a list of keys with new values at specific state commitment (update) and returns a new state commitment
	Set(update *Update) (newStateCommitment StateCommitment, err error)

	// Prove returns proofs for the given keys at specific state commitment
	Prove(query *Query) (proof Proof, err error)

	// returns an approximate size of memory used by the ledger
	MemSize() (int64, error)

	// returns an approximate size of disk used by the ledger
	DiskSize() (int64, error)
}

// Query holds all data needed for a ledger read or ledger proof
type Query struct {
	stateCommitment StateCommitment
	keys            []Key
}

// NewQuery constructs a new ledger  query
func NewQuery(sc StateCommitment, keys []Key) (*Query, error) {
	return &Query{stateCommitment: sc, keys: keys}, nil
}

// Keys returns keys of the query
func (q *Query) Keys() []Key {
	return q.keys
}

// Size returns number of keys in the query
func (q *Query) Size() int {
	return len(q.keys)
}

// StateCommitment returns the state commitment of this query
func (q *Query) StateCommitment() StateCommitment {
	return q.stateCommitment
}

// Update holds all data needed for a ledger update
type Update struct {
	stateCommitment StateCommitment
	keys            []Key
	values          []Value
}

// Size returns number of keys in the ledger update
func (u *Update) Size() int {
	return len(u.keys)
}

// Keys returns keys of the update
func (u *Update) Keys() []Key {
	return u.keys
}

// Values returns value of the update
func (u *Update) Values() []Value {
	return u.values
}

// StateCommitment returns the state commitment of this update
func (u *Update) StateCommitment() StateCommitment {
	return u.stateCommitment
}

// appendKV append a key value pair to the update
func (u *Update) appendKV(key *Key, value Value) {
	u.keys = append(u.keys, *key)
}

// NewEmptyUpdate returns an empty ledger update
func NewEmptyUpdate(sc StateCommitment) (*Update, error) {
	return &Update{stateCommitment: sc}, nil
}

// NewUpdate returns an ledger update
func NewUpdate(sc StateCommitment, keys []Key, values []Value) (*Update, error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("length mismatch: keys have %d elements, but values have %d elements", len(keys), len(values))
	}
	return &Update{stateCommitment: sc, keys: keys, values: values}, nil

}

// StateCommitment captures a commitment to an state of the ledger
type StateCommitment []byte

func (sc StateCommitment) String() string {
	return hex.EncodeToString(sc)
}

// Equals compares the state commitment to another state commitment
func (sc StateCommitment) Equals(o StateCommitment) bool {
	if o == nil {
		return false
	}
	return bytes.Equal(sc, o)
}

// Proof is a byte slice capturing encoded version of a batch proof
type Proof []byte

func (pr Proof) String() string {
	return hex.EncodeToString(pr)
}

// Equals compares a proof to another ledger proof
func (pr Proof) Equals(o Proof) bool {
	if o == nil {
		return false
	}
	return bytes.Equal(pr, o)
}

// Key represents a hierarchical ledger key
type Key struct {
	KeyParts []KeyPart
}

// NewKey construct a new key
func NewKey(kp []KeyPart) *Key {
	return &Key{KeyParts: kp}
}

// Size returns the byte size needed to encode the key
func (k *Key) Size() int {
	size := 0
	for _, kp := range k.KeyParts {
		// value size + 2 bytes for type
		size += len(kp.Value) + 2
	}
	return size
}

func (k *Key) String() string {
	// TODO include type
	ret := ""
	for _, kp := range k.KeyParts {
		ret += string(kp.Value)
	}
	return ret
}

// Equals compares this key to another key
func (k *Key) Equals(other *Key) bool {
	if other == nil {
		return false
	}
	if len(k.KeyParts) != len(other.KeyParts) {
		return false
	}
	for i, kp := range k.KeyParts {
		if !kp.Equals(&other.KeyParts[i]) {
			return false
		}
	}
	return true
}

// KeyPart is a typed part of a key
type KeyPart struct {
	Type  uint16
	Value []byte
}

// NewKeyPart construct a new key part
func NewKeyPart(typ uint16, val []byte) *KeyPart {
	return &KeyPart{Type: typ, Value: val}
}

// Equals compares this key part to another key part
func (kp *KeyPart) Equals(other *KeyPart) bool {
	if other == nil {
		return false
	}
	if kp.Type != other.Type {
		return false
	}
	return bytes.Equal(kp.Value, other.Value)
}

// Value holds the value part of a ledger key value pair
type Value []byte

// Size returns the value size
func (v Value) Size() int {
	return len(v)
}

func (v Value) String() string {
	return hex.EncodeToString(v)
}

// Equals compares a ledger Value to another one
func (v Value) Equals(other Value) bool {
	if other == nil {
		return false
	}
	return bytes.Equal(v, other)
}
