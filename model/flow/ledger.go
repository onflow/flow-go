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

func (r *RegisterID) String() string {
	const all string = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff"
	ownerLen := len(r.Owner)
	controllerLen := len(r.Controller)
	keyLen := len(r.Key)

	requiredLen := ((ownerLen + controllerLen + keyLen) * 2) + 2

	arr := make([]byte, requiredLen, requiredLen)
	n := 0

	for i := 0; i < ownerLen; i++ {
		arr[n] = all[int(r.Owner[i])*2]
		n++
		arr[n] = all[(int(r.Owner[i])*2)+1]
		n++
	}

	arr[n] = byte('/')
	n++

	for j := 0; j < controllerLen; j++ {
		arr[n] = all[(int(r.Controller[j])*2)+1]
		n++
		arr[n] = all[(int(r.Controller[j])*2)+1]
		n++
	}

	arr[n] = byte('/')
	n++

	for k := 0; k < keyLen; k++ {
		arr[n] = all[int(r.Key[k])*2]
		n++
		arr[n] = all[(int(r.Key[k])*2)+1]
		n++
	}
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
