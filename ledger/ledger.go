package ledger

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

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

	// PathFinderVersion returns the pathfinder version of the ledger
	PathFinderVersion() uint8

	// GetSingleValue returns value for a given key at specific state
	GetSingleValue(query *TrieReadSingleValue) (value Value, err error)

	// Get returns values for the given slice of keys at specific state
	Get(query *TrieRead) (values []Value, err error)

	// Update updates a list of keys with new values at specific state (update) and returns a new state
	Set(update *TrieUpdate) (newState State, err error)

	// Prove returns proofs for the given keys at specific state
	Prove(query *TrieRead) (proof Proof, err error)
}

const (
	keyPartOwner      = uint16(0)
	keyPartController = uint16(1)
	keyPartKey        = uint16(2)
)

// KeyID represents a ledger key consisting of owner, controller, and key.
// KeyID is a simplified ledger key that requires fewer heap allocations
// than ledger.Key.
type KeyID struct {
	Owner      string
	Controller string
	Key        string
}

// NewKeyID returns a new key ID.
func NewKeyID(owner, controller, key string) KeyID {
	return KeyID{
		Owner:      owner,
		Controller: controller,
		Key:        key,
	}
}

// CanonicalForm returns a byte slice describing the key ID.
// WARNING:
// - KeyID.CanonicalForm() and KeyIDToKey(k).CanonicalForm() must
//   produce the same result. Changes to these functions must be in sync.
// - Changing this function impacts how leaf hashes are computed.
// - Don't use this to reconstruct the key later.
func (k *KeyID) CanonicalForm() []byte {
	const (
		keyPartCount         = 3
		encodedKeyPartLength = 3
		encodedKeyLength     = keyPartCount * encodedKeyPartLength
	)

	length := len(k.Owner) + len(k.Controller) + len(k.Key) + encodedKeyLength

	b := make([]byte, 0, length)

	// encode owner
	b = append(b, byte('/'), 0x30, byte('/')) // 0x30 = strconv.Itoa(keyPartOwner)
	b = append(b, k.Owner...)

	// encode controller
	b = append(b, byte('/'), 0x31, byte('/')) // 0x31 = strconv.Itoa(keyPartController)
	b = append(b, k.Controller...)

	// encode key
	b = append(b, byte('/'), 0x32, byte('/')) // 0x32 = strconv.Itoa(keyPartKey)
	b = append(b, k.Key...)

	return b
}

// KeyIDToKey returns ledger.Key from ledger.KeyID.
func KeyIDToKey(reg KeyID) Key {
	return NewKey([]KeyPart{
		NewKeyPart(keyPartOwner, []byte(reg.Owner)),
		NewKeyPart(keyPartController, []byte(reg.Controller)),
		NewKeyPart(keyPartKey, []byte(reg.Key)),
	})
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
// WARNING:
// - KeyID.CanonicalForm() and KeyIDToKey(k).CanonicalForm() must
//   produce the same result. Changes to these functions must be in sync.
// - Changing this function impacts how leaf hashes are computed.
// - Don't use this to reconstruct the key later.
func (k *Key) CanonicalForm() []byte {
	// calculate the size of the byte array

	// the maximum size of a uint16 is 5 characters, so
	// this is using 10 for the estimate, to include the two '/'
	// characters and an extra 3 characters for padding safety

	constant := 10

	requiredLen := constant * len(k.KeyParts)
	for _, kp := range k.KeyParts {
		requiredLen += len(kp.Value)
	}

	retval := make([]byte, 0, requiredLen)

	for _, kp := range k.KeyParts {
		typeNumber := strconv.Itoa(int(kp.Type))

		retval = append(retval, byte('/'))
		retval = append(retval, []byte(typeNumber)...)
		retval = append(retval, byte('/'))
		retval = append(retval, kp.Value...)
	}

	// create a byte slice with the correct size and copy
	// the estimated string into it.
	return retval
}

func (k *Key) String() string {
	return string(k.CanonicalForm())
}

// DeepCopy returns a deep copy of the key
func (k *Key) DeepCopy() Key {
	newKPs := make([]KeyPart, len(k.KeyParts))
	for i, kp := range k.KeyParts {
		newKPs[i] = *kp.DeepCopy()
	}
	return Key{KeyParts: newKPs}
}

// Equals compares this key to another key
// A nil key is equivalent to an empty key.
func (k *Key) Equals(other *Key) bool {
	if k == nil || len(k.KeyParts) == 0 {
		return other == nil || len(other.KeyParts) == 0
	}
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
	newV := make([]byte, len(kp.Value))
	copy(newV, kp.Value)
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
	newV := make([]byte, len(v))
	copy(newV, v)
	return newV
}

// Equals compares a ledger Value to another one
// A nil value is equivalent to an empty value.
func (v Value) Equals(other Value) bool {
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
