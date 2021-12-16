package ledger

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/module"
)

// Ledger is a stateful fork-aware key/value storage.
// Any update (value change for a key) to the ledger generates a new ledger state.
// Updates can be applied to any recent states. These changes don't have to be sequential and ledger supports a tree of states.
// Ledger provides value lookup by key at a particular state (historic lookups) and can prove the existence/non-existence of a key-value pair at the given state.
// Ledger assumes the initial state includes all keys with an empty bytes slice as value.
type Ledger interface {
	// ledger implements methods needed to be ReadyDone aware
	module.ReadyDoneAware

	// InitialState returns the initial state of the ledger
	InitialState() State

	// Get returns values for the given slice of keys at specific state
	Get(query *Query) (values []Value, err error)

	// Update updates a list of keys with new values at specific state (update) and returns a new state
	Set(update *Update) (newState State, trieUpdate *TrieUpdate, err error)

	// Prove returns proofs for the given keys at specific state
	Prove(query *Query) (proof Proof, err error)
}

// Query holds all data needed for a ledger read or ledger proof
type Query struct {
	state State
	keys  []Key
}

// NewEmptyQuery returns an empty ledger query
func NewEmptyQuery(sc State) (*Query, error) {
	return &Query{state: sc}, nil
}

// NewQuery constructs a new ledger  query
func NewQuery(sc State, keys []Key) (*Query, error) {
	return &Query{state: sc, keys: keys}, nil
}

// Keys returns keys of the query
func (q *Query) Keys() []Key {
	return q.keys
}

// Size returns number of keys in the query
func (q *Query) Size() int {
	return len(q.keys)
}

// State returns the state part of the query
func (q *Query) State() State {
	return q.state
}

// SetState sets the state part of the query
func (q *Query) SetState(s State) {
	q.state = s
}

// Update holds all data needed for a ledger update
type Update struct {
	state  State
	keys   []Key
	values []Value
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

// State returns the state part of this update
func (u *Update) State() State {
	return u.state
}

// SetState sets the state part of the update
func (u *Update) SetState(sc State) {
	u.state = sc
}

// NewEmptyUpdate returns an empty ledger update
func NewEmptyUpdate(sc State) (*Update, error) {
	return &Update{state: sc}, nil
}

// NewUpdate returns an ledger update
func NewUpdate(sc State, keys []Key, values []Value) (*Update, error) {
	if len(keys) != len(values) {
		return nil, fmt.Errorf("length mismatch: keys have %d elements, but values have %d elements", len(keys), len(values))
	}
	return &Update{state: sc, keys: keys, values: values}, nil

}

// State captures an state of the ledger
type State hash.Hash

// DummyState is an arbitrary value used in function failure cases,
// although it can represent a valid state.
var DummyState = State(hash.DummyHash)

// String returns the hex encoding of the state
func (sc State) String() string {
	return hex.EncodeToString(sc[:])
}

// Base64 return the base64 encoding of the state
func (sc State) Base64() string {
	return base64.StdEncoding.EncodeToString(sc[:])
}

// Equals compares the state to another state
func (sc State) Equals(o State) bool {
	return sc == o
}

// ToState converts a byte slice into a State.
// It returns an error if the slice has an invalid length.
func ToState(stateBytes []byte) (State, error) {
	var state State
	if len(stateBytes) != len(state) {
		return DummyState, fmt.Errorf("expecting %d bytes but got %d bytes", len(state), len(stateBytes))
	}
	copy(state[:], stateBytes)
	return state, nil
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
func NewKey(kp []KeyPart) Key {
	return Key{KeyParts: kp}
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

// CanonicalForm returns a byte slice describing the key
// Warning, Changing this has an impact on how leaf hashes are computed
// don't use this to reconstruct the key later
func (k *Key) CanonicalForm() []byte {
	ret := ""
	for _, kp := range k.KeyParts {
		ret += fmt.Sprintf("/%d/%v", kp.Type, string(kp.Value))
	}
	return []byte(ret)
}

func (k *Key) String() string {
	return string(k.CanonicalForm())
}

// DeepCopy returns a deep copy of the key
func (k *Key) DeepCopy() Key {
	newKPs := make([]KeyPart, 0, len(k.KeyParts))
	for _, kp := range k.KeyParts {
		newKPs = append(newKPs, *kp.DeepCopy())
	}
	return Key{KeyParts: newKPs}
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
func NewKeyPart(typ uint16, val []byte) KeyPart {
	return KeyPart{Type: typ, Value: val}
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

// DeepCopy returns a deep copy of the key part
func (kp *KeyPart) DeepCopy() *KeyPart {
	newV := make([]byte, 0, len(kp.Value))
	newV = append(newV, kp.Value...)
	return &KeyPart{Type: kp.Type, Value: newV}
}

func (kp KeyPart) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type  uint16
		Value string
	}{
		Type:  kp.Type,
		Value: hex.EncodeToString(kp.Value),
	})
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

// DeepCopy returns a deep copy of the value
func (v Value) DeepCopy() Value {
	newV := make([]byte, 0, len(v))
	newV = append(newV, []byte(v)...)
	return newV
}

// Equals compares a ledger Value to another one
func (v Value) Equals(other Value) bool {
	if other == nil {
		return false
	}
	return bytes.Equal(v, other)
}

func (v Value) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(v))
}

// Migration defines how to convert the given slice of input payloads into an slice of output payloads
type Migration func(payloads []Payload) ([]Payload, error)

// Reporter reports on data from the state
type Reporter interface {
	// Name returns the name of the reporter. Only used for logging.
	Name() string
	// Report accepts slice ledger payloads and reports the state of the ledger
	Report(payloads []Payload) error
}
