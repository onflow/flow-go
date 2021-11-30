package flow

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/model/fingerprint"
)

type RegisterID struct {
	Owner      string
	Controller string
	Key        string
}

// this function returns a string format of a RegisterID in the form '%x/%x/%x'
// it has been optimized to avoid the memory allocations inside Sprintf
func (r *RegisterID) String() string {
	ownerLen := len(r.Owner)
	controllerLen := len(r.Controller)

	requiredLen := ((ownerLen + controllerLen + keyLen) * 2) + 2

	arr := make([]byte, requiredLen)

	hex.Encode(arr, []byte(r.Owner))

	arr[2*ownerLen] = byte('/')

	hex.Encode(arr[(2*ownerLen)+1:], []byte(r.Controller))

	arr[2*(ownerLen+controllerLen)+1] = byte('/')

	hex.Encode(arr[2*(ownerLen+controllerLen+1):], []byte(r.Key))

	return string(arr)
}

// Bytes returns a bytes representation of the RegisterID.
//
// The encoding uses the injective fingerprint module.
func (r *RegisterID) Bytes() []byte {
	return fingerprint.Fingerprint(r)
}

func NewRegisterID(owner, controller, key string) RegisterID {
	return RegisterID{
		Owner:      owner,
		Controller: controller,
		Key:        key,
	}
}

// RegisterValue (value part of Register)
type RegisterValue = []byte

type RegisterEntry struct {
	Key   RegisterID
	Value RegisterValue
}

//handy container for sorting
type RegisterEntries []RegisterEntry

func (d RegisterEntries) Len() int {
	return len(d)
}

func (d RegisterEntries) Less(i, j int) bool {
	if d[i].Key.Owner != d[j].Key.Owner {
		return d[i].Key.Owner < d[j].Key.Owner
	} else if d[i].Key.Controller != d[j].Key.Controller {
		return d[i].Key.Controller < d[j].Key.Controller
	}
	return d[i].Key.Key < d[j].Key.Key
}

func (d RegisterEntries) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func (d RegisterEntries) IDs() []RegisterID {
	r := make([]RegisterID, len(d))
	for i, entry := range d {
		r[i] = entry.Key
	}
	return r
}

func (d RegisterEntries) Values() []RegisterValue {
	r := make([]RegisterValue, len(d))
	for i, entry := range d {
		r[i] = entry.Value
	}
	return r
}

// StorageProof (proof of a read or update to the state, Merkle path of some sort)
type StorageProof = []byte

// StateCommitment holds the root hash of the tree (Snapshot)
// TODO: solve the circular dependency and define StateCommitment as ledger.State
type StateCommitment hash.Hash

// DummyStateCommitment is an arbitrary value used in function failure cases,
// although it can represent a valid state commitment.
var DummyStateCommitment = StateCommitment(hash.DummyHash)

// ToStateCommitment converts a byte slice into a StateComitment.
// It returns an error if the slice has an invalid length.
func ToStateCommitment(stateBytes []byte) (StateCommitment, error) {
	var state StateCommitment
	if len(stateBytes) != len(state) {
		return DummyStateCommitment, fmt.Errorf("expecting %d bytes but got %d bytes", len(state), len(stateBytes))
	}
	copy(state[:], stateBytes)
	return state, nil
}

func (s StateCommitment) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(s[:]))
}

func (s *StateCommitment) UnmarshalJSON(data []byte) error {
	var stateCommitmentHex string
	if err := json.Unmarshal(data, &stateCommitmentHex); err != nil {
		return err
	}
	b, err := hex.DecodeString(stateCommitmentHex)
	if err != nil {
		return err
	}
	h, err := hash.ToHash(b)
	if err != nil {
		return err
	}
	*s = StateCommitment(h)
	return nil
}
