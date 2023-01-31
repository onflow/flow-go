package flow

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/model/fingerprint"
)

const (
	// Service level keys (owner is empty):
	UUIDKey         = "uuid"
	AddressStateKey = "account_address_state"

	// Account level keys
	AccountKeyPrefix   = "a."
	AccountStatusKey   = AccountKeyPrefix + "s"
	CodeKeyPrefix      = "code."
	ContractNamesKey   = "contract_names"
	PublicKeyKeyPrefix = "public_key_"
)

func addressToOwner(address Address) string {
	return string(address.Bytes())
}

type RegisterID struct {
	Owner string
	Key   string
}

var AddressStateRegisterID = RegisterID{
	Owner: "",
	Key:   AddressStateKey,
}

var UUIDRegisterID = RegisterID{
	Owner: "",
	Key:   UUIDKey,
}

func AccountStatusRegisterID(address Address) RegisterID {
	return RegisterID{
		Owner: addressToOwner(address),
		Key:   AccountStatusKey,
	}
}

func PublicKeyRegisterID(address Address, index uint64) RegisterID {
	return RegisterID{
		Owner: addressToOwner(address),
		Key:   fmt.Sprintf("public_key_%d", index),
	}
}

func ContractNamesRegisterID(address Address) RegisterID {
	return RegisterID{
		Owner: addressToOwner(address),
		Key:   ContractNamesKey,
	}
}

func ContractRegisterID(address Address, contractName string) RegisterID {
	return RegisterID{
		Owner: addressToOwner(address),
		Key:   CodeKeyPrefix + contractName,
	}
}

func CadenceRegisterID(owner []byte, key []byte) RegisterID {
	return RegisterID{
		Owner: string(BytesToAddress(owner).Bytes()),
		Key:   string(key),
	}
}

// IsInternalState returns true if the register id is controlled by flow-go and
// return false otherwise (key controlled by the cadence env).
func (id RegisterID) IsInternalState() bool {
	// check if is a service level key (owner is empty)
	// cases:
	//      - "", "uuid"
	//      - "", "account_address_state"
	if len(id.Owner) == 0 && (id.Key == UUIDKey || id.Key == AddressStateKey) {
		return true
	}

	// check account level keys
	// cases:
	//      - address, "contract_names"
	//      - address, "code.%s" (contract name)
	//      - address, "public_key_%d" (index)
	//      - address, "a.s" (account status)
	return strings.HasPrefix(id.Key, PublicKeyKeyPrefix) ||
		id.Key == ContractNamesKey ||
		strings.HasPrefix(id.Key, CodeKeyPrefix) ||
		id.Key == AccountStatusKey
}

// IsSlabIndex returns true if the key is a slab index for an account's ordered fields
// map.
//
// In general, each account's regular fields are stored in ordered map known
// only to cadence.  Cadence encodes this map into bytes and split the bytes
// into slab chunks before storing the slabs into the ledger.
func (id RegisterID) IsSlabIndex() bool {
	return len(id.Key) == 9 && id.Key[0] == '$'
}

// TODO(patrick): pretty print flow internal register ids.
// String returns formatted string representation of the RegisterID.
func (id RegisterID) String() string {
	formattedKey := ""
	if id.IsSlabIndex() {
		i := uint64(binary.BigEndian.Uint64([]byte(id.Key[1:])))
		formattedKey = fmt.Sprintf("$%d", i)
	} else {
		formattedKey = fmt.Sprintf("#%x", []byte(id.Key))
	}

	return fmt.Sprintf("%x/%s", id.Owner, formattedKey)
}

// Bytes returns a bytes representation of the RegisterID.
//
// The encoding uses the injective fingerprint module.
func (r *RegisterID) Bytes() []byte {
	return fingerprint.Fingerprint(r)
}

func NewRegisterID(owner, key string) RegisterID {
	return RegisterID{
		Owner: owner,
		Key:   key,
	}
}

// RegisterValue (value part of Register)
type RegisterValue = []byte

type RegisterEntry struct {
	Key   RegisterID
	Value RegisterValue
}

// handy container for sorting
type RegisterEntries []RegisterEntry

func (d RegisterEntries) Len() int {
	return len(d)
}

func (d RegisterEntries) Less(i, j int) bool {
	if d[i].Key.Owner != d[j].Key.Owner {
		return d[i].Key.Owner < d[j].Key.Owner
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

// ToStateCommitment converts a byte slice into a StateCommitment.
// It returns an error if the slice has an invalid length.
// The returned error indicates that the given byte slice is not a
// valid root hash of an execution state.  As the function is
// side-effect free, all failures are simply a no-op.
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
	// first, attempt to unmarshal assuming data is a hex string representation
	err := s.unmarshalJSONHexString(data)
	if err == nil {
		return nil
	}
	// fallback to unmarshalling as [32]byte
	return s.unmarshalJSONByteArr(data)
}

func (s *StateCommitment) unmarshalJSONHexString(data []byte) error {
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

func (s *StateCommitment) unmarshalJSONByteArr(data []byte) error {
	var stateCommitment [32]byte
	if err := json.Unmarshal(data, &stateCommitment); err != nil {
		return err
	}
	*s = stateCommitment
	return nil
}
