package ledger

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fxamacker/cbor/v2"

	cryptoHash "github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/hash"
)

// Path captures storage path of a payload;
// where we store a payload in the ledger
type Path hash.Hash

func (p Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(hex.EncodeToString(p[:]))
}

// DummyPath is an arbitrary path value, used in function error returns.
var DummyPath = Path(hash.DummyHash)

// PathLen is the size of paths in bytes.
const PathLen = 32

// The node maximum height or the tree height.
// It corresponds to the path size in bits.
const NodeMaxHeight = PathLen * 8

// we are currently supporting paths of a size equal to 32 bytes.
// I.e. path length from the rootNode of a fully expanded tree to the leaf node is 256.
// A path of length k is comprised of k+1 vertices. Hence, we need 257 default hashes.
const defaultHashesNum = NodeMaxHeight + 1

// array to store all default hashes
var defaultHashes [defaultHashesNum]hash.Hash

func init() {

	// default value and default hash value for a default node
	var defaultLeafHash hash.Hash
	castedPointer := (*[hash.HashLen]byte)(&defaultLeafHash)
	cryptoHash.ComputeSHA3_256(castedPointer, []byte("default:"))

	// Creates the Default hashes from base to level height
	defaultHashes[0] = defaultLeafHash
	for i := 1; i < defaultHashesNum; i++ {
		defaultHashes[i] = hash.HashInterNode(defaultHashes[i-1], defaultHashes[i-1])
	}
}

// GetDefaultHashForHeight returns the default hashes of the SMT at a specified height.
//
// For each tree level N, there is a default hash equal to the chained
// hashing of the default value N times.
func GetDefaultHashForHeight(height int) hash.Hash {
	return defaultHashes[height]
}

// ComputeCompactValue computes the value for the node considering the sub tree
// to only include this value and default values. It writes the hash result to the result input.
// UNCHECKED: payload!= nil
func ComputeCompactValue(path hash.Hash, value []byte, nodeHeight int) hash.Hash {
	// if register is unallocated: return default hash
	if len(value) == 0 {
		return GetDefaultHashForHeight(nodeHeight)
	}

	var out hash.Hash
	out = hash.HashLeaf(path, value)   // we first compute the hash of the fully-expanded leaf
	for h := 1; h <= nodeHeight; h++ { // then, we hash our way upwards towards the root until we hit the specified nodeHeight
		// h is the height of the node, whose hash we are computing in this iteration.
		// The hash is computed from the node's children at height h-1.
		bit := bitutils.ReadBit(path[:], NodeMaxHeight-h)
		if bit == 1 { // right branching
			out = hash.HashInterNode(GetDefaultHashForHeight(h-1), out)
		} else { // left branching
			out = hash.HashInterNode(out, GetDefaultHashForHeight(h-1))
		}
	}
	return out
}

// TrieRead captures a trie read query
type TrieRead struct {
	RootHash RootHash
	Paths    []Path
}

// TrieReadSinglePayload contains trie read query for a single payload
type TrieReadSingleValue struct {
	RootHash RootHash
	Path     Path
}

// TrieUpdate holds all data for a trie update
type TrieUpdate struct {
	RootHash RootHash
	Paths    []Path
	Payloads []*Payload
}

// Size returns number of paths in the trie update
func (u *TrieUpdate) Size() int {
	return len(u.Paths)
}

// IsEmpty returns true if key or value is not empty
func (u *TrieUpdate) IsEmpty() bool {
	return u.Size() == 0
}

func (u *TrieUpdate) String() string {
	str := "Trie Update:\n "
	str += "\t triehash : " + u.RootHash.String() + "\n"
	for i, p := range u.Paths {
		str += fmt.Sprintf("\t\t path %d : %s\n", i, p)
	}
	str += fmt.Sprintf("\t paths len: %d , bytesize: %d\n", len(u.Paths), len(u.Paths)*PathLen)
	tp := 0
	for _, p := range u.Payloads {
		tp += p.Size()
	}
	str += fmt.Sprintf("\t total size of payloads : %d \n", tp)
	return str
}

// Equals compares this trie update to another trie update
func (u *TrieUpdate) Equals(other *TrieUpdate) bool {
	if other == nil {
		return false
	}
	if !u.RootHash.Equals(other.RootHash) {
		return false
	}
	if len(u.Paths) != len(other.Paths) {
		return false
	}
	for i := range u.Paths {
		if !u.Paths[i].Equals(other.Paths[i]) {
			return false
		}
	}

	if len(u.Payloads) != len(other.Payloads) {
		return false
	}
	for i := range u.Payloads {
		if !u.Payloads[i].Equals(other.Payloads[i]) {
			return false
		}
	}
	return true
}

// RootHash captures the root hash of a trie
type RootHash hash.Hash

func (rh RootHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(rh.String())
}

func (rh RootHash) String() string {
	return hex.EncodeToString(rh[:])
}

// Equals compares the root hash to another one
func (rh RootHash) Equals(o RootHash) bool {
	return rh == o
}

// ToRootHash converts a byte slice into a root hash.
// It returns an error if the slice has an invalid length.
func ToRootHash(rootHashBytes []byte) (RootHash, error) {
	var rootHash RootHash
	if len(rootHashBytes) != len(rootHash) {
		return RootHash(hash.DummyHash), fmt.Errorf("expecting %d bytes but got %d bytes", len(rootHash), len(rootHashBytes))
	}
	copy(rootHash[:], rootHashBytes)
	return rootHash, nil
}

func (p Path) String() string {
	str := ""
	for _, i := range p {
		str += fmt.Sprintf("%08b", i)
	}
	str = str[0:8] + "..." + str[len(str)-8:]
	return str
}

// Equals compares this path to another path
func (p Path) Equals(o Path) bool {
	return p == o
}

// ToPath converts a byte slice into a path.
// It returns an error if the slice has an invalid length.
func ToPath(pathBytes []byte) (Path, error) {
	var path Path
	if len(pathBytes) != len(path) {
		return DummyPath, fmt.Errorf("expecting %d bytes but got %d bytes", len(path), len(pathBytes))
	}
	copy(path[:], pathBytes)
	return path, nil
}

// encKey represents an encoded ledger key.
type encKey []byte

// Size returns the byte size of the encoded key.
func (k encKey) Size() int {
	return len(k)
}

// String returns the string representation of the encoded key.
func (k encKey) String() string {
	return hex.EncodeToString(k)
}

// Equals compares this encoded key to another encoded key.
// A nil encoded key is equivalent to an empty encoded key.
func (k encKey) Equals(other encKey) bool {
	return bytes.Equal(k, other)
}

// DeepCopy returns a deep copy of the encoded key.
func (k encKey) DeepCopy() encKey {
	newK := make([]byte, len(k))
	copy(newK, k)
	return newK
}

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	// encKey is key encoded using PayloadVersion.
	// Version and type data are not encoded to save 3 bytes.
	// NOTE: encKey translates to Key{} when encKey is
	//       one of these three values:
	//       nil, []byte{}, []byte{0,0}.
	encKey encKey
	value  Value
}

// serializablePayload is used to serialize ledger.Payload.
// Encoder only serializes exported fields and ledger.Payload's
// key and value fields are not exported.  So it is necessary to
// use serializablePayload for encoding.
type serializablePayload struct {
	Key   Key
	Value Value
}

// MarshalJSON returns JSON encoding of p.
func (p Payload) MarshalJSON() ([]byte, error) {
	k, err := p.Key()
	if err != nil {
		return nil, err
	}
	sp := serializablePayload{Key: k, Value: p.value}
	return json.Marshal(sp)
}

// UnmarshalJSON unmarshals a JSON value of payload.
func (p *Payload) UnmarshalJSON(b []byte) error {
	if p == nil {
		return errors.New("UnmarshalJSON on nil Payload")
	}
	var sp serializablePayload
	if err := json.Unmarshal(b, &sp); err != nil {
		return err
	}
	p.encKey = encodeKey(&sp.Key, PayloadVersion)
	p.value = sp.Value
	return nil
}

// MarshalCBOR returns CBOR encoding of p.
func (p Payload) MarshalCBOR() ([]byte, error) {
	k, err := p.Key()
	if err != nil {
		return nil, err
	}
	sp := serializablePayload{Key: k, Value: p.value}
	return cbor.Marshal(sp)
}

// UnmarshalCBOR unmarshals a CBOR value of payload.
func (p *Payload) UnmarshalCBOR(b []byte) error {
	if p == nil {
		return errors.New("UnmarshalCBOR on nil payload")
	}
	var sp serializablePayload
	if err := cbor.Unmarshal(b, &sp); err != nil {
		return err
	}
	p.encKey = encodeKey(&sp.Key, PayloadVersion)
	p.value = sp.Value
	return nil
}

// Key returns payload key.
// Error indicates that ledger.Key can't be created from payload key, so
// migration and reporting (known callers) should abort.
// CAUTION: do not modify returned key because it shares underlying data with payload key.
func (p *Payload) Key() (Key, error) {
	if p == nil || len(p.encKey) == 0 {
		return Key{}, nil
	}
	k, err := decodeKey(p.encKey, true, PayloadVersion)
	if err != nil {
		return Key{}, err
	}
	return *k, nil
}

// Value returns payload value.
// CAUTION: do not modify returned value because it shares underlying data with payload value.
func (p *Payload) Value() Value {
	if p == nil {
		return Value{}
	}
	return p.value
}

// Size returns the size of the payload
func (p *Payload) Size() int {
	if p == nil {
		return 0
	}
	return p.encKey.Size() + p.value.Size()
}

// IsEmpty returns true if payload is nil or value is empty
func (p *Payload) IsEmpty() bool {
	return p == nil || p.value.Size() == 0
}

// TODO fix me
func (p *Payload) String() string {
	// TODO improve this key, values
	return p.encKey.String() + " " + p.value.String()
}

// Equals compares this payload to another payload
// A nil payload is equivalent to an empty payload.
func (p *Payload) Equals(other *Payload) bool {
	if p == nil || (p.encKey.Size() == 0 && p.value.Size() == 0) {
		return other == nil || (other.encKey.Size() == 0 && other.value.Size() == 0)
	}
	if other == nil {
		return false
	}
	return p.encKey.Equals(other.encKey) && p.value.Equals(other.value)
}

// ValueEquals compares this payload value to another payload value.
// A nil payload is equivalent to an empty payload.
// NOTE: prefer using this function over payload.Value.Equals()
// when comparing payload values.  payload.ValueEquals() handles
// nil payload, while payload.Value.Equals() panics on nil payload.
func (p *Payload) ValueEquals(other *Payload) bool {
	pEmpty := p.IsEmpty()
	otherEmpty := other.IsEmpty()
	if pEmpty != otherEmpty {
		// Only one payload is empty
		return false
	}
	if pEmpty {
		// Both payloads are empty
		return true
	}
	// Compare values since both payloads are not empty.
	return p.value.Equals(other.value)
}

// DeepCopy returns a deep copy of the payload
func (p *Payload) DeepCopy() *Payload {
	if p == nil {
		return nil
	}
	k := p.encKey.DeepCopy()
	v := p.value.DeepCopy()
	return &Payload{encKey: k, value: v}
}

// NewPayload returns a new payload
func NewPayload(key Key, value Value) *Payload {
	ek := encodeKey(&key, PayloadVersion)
	return &Payload{encKey: ek, value: value}
}

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}

// TrieProof includes all the information needed to walk
// through a trie branch from an specific leaf node (key)
// up to the root of the trie.
type TrieProof struct {
	Path      Path        // path
	Payload   *Payload    // payload
	Interims  []hash.Hash // the non-default intermediate nodes in the proof
	Inclusion bool        // flag indicating if this is an inclusion or exclusion proof
	Flags     []byte      // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8       // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewTrieProof creates a new instance of Trie Proof
func NewTrieProof() *TrieProof {
	return &TrieProof{
		Payload:   EmptyPayload(),
		Interims:  make([]hash.Hash, 0),
		Inclusion: false,
		Flags:     make([]byte, PathLen),
		Steps:     0,
	}
}

func (p *TrieProof) String() string {
	flagStr := ""
	for _, f := range p.Flags {
		flagStr += fmt.Sprintf("%08b", f)
	}
	proofStr := fmt.Sprintf("size: %d flags: %v\n", p.Steps, flagStr)
	proofStr += fmt.Sprintf("\t path: %v payload: %v\n", p.Path, p.Payload)

	if p.Inclusion {
		proofStr += "\t inclusion proof:\n"
	} else {
		proofStr += "\t noninclusion proof:\n"
	}
	interimIndex := 0
	for j := 0; j < int(p.Steps); j++ {
		// if bit is set
		if p.Flags[j/8]&(1<<(7-j%8)) != 0 {
			proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, p.Interims[interimIndex])
			interimIndex++
		}
	}
	return proofStr
}

// Equals compares this proof to another payload
func (p *TrieProof) Equals(o *TrieProof) bool {
	if o == nil {
		return false
	}
	if !p.Path.Equals(o.Path) {
		return false
	}
	if !p.Payload.Equals(o.Payload) {
		return false
	}
	if len(p.Interims) != len(o.Interims) {
		return false
	}
	for i, inter := range p.Interims {
		if inter != o.Interims[i] {
			return false
		}
	}
	if p.Inclusion != o.Inclusion {
		return false
	}
	if !bytes.Equal(p.Flags, o.Flags) {
		return false
	}
	if p.Steps != o.Steps {
		return false
	}
	return true
}

// TrieBatchProof is a struct that holds the proofs for several keys
//
// so there is no need for two calls (read, proofs)
type TrieBatchProof struct {
	Proofs []*TrieProof
}

// NewTrieBatchProof creates a new instance of BatchProof
func NewTrieBatchProof() *TrieBatchProof {
	bp := new(TrieBatchProof)
	bp.Proofs = make([]*TrieProof, 0)
	return bp
}

// NewTrieBatchProofWithEmptyProofs creates an instance of Batchproof
// filled with n newly created proofs (empty)
func NewTrieBatchProofWithEmptyProofs(numberOfProofs int) *TrieBatchProof {
	bp := new(TrieBatchProof)
	bp.Proofs = make([]*TrieProof, numberOfProofs)
	for i := 0; i < numberOfProofs; i++ {
		bp.Proofs[i] = NewTrieProof()
	}
	return bp
}

// Size returns the number of proofs
func (bp *TrieBatchProof) Size() int {
	return len(bp.Proofs)
}

// Paths returns the slice of paths for this batch proof
func (bp *TrieBatchProof) Paths() []Path {
	paths := make([]Path, len(bp.Proofs))
	for i, p := range bp.Proofs {
		paths[i] = p.Path
	}
	return paths
}

// Payloads returns the slice of paths for this batch proof
func (bp *TrieBatchProof) Payloads() []*Payload {
	payloads := make([]*Payload, len(bp.Proofs))
	for i, p := range bp.Proofs {
		payloads[i] = p.Payload
	}
	return payloads
}

func (bp *TrieBatchProof) String() string {
	res := fmt.Sprintf("trie batch proof includes %d proofs: \n", bp.Size())
	for _, proof := range bp.Proofs {
		res = res + "\n" + proof.String()
	}
	return res
}

// AppendProof adds a proof to the batch proof
func (bp *TrieBatchProof) AppendProof(p *TrieProof) {
	bp.Proofs = append(bp.Proofs, p)
}

// MergeInto adds all of its proofs into the dest batch proof
func (bp *TrieBatchProof) MergeInto(dest *TrieBatchProof) {
	for _, p := range bp.Proofs {
		dest.AppendProof(p)
	}
}

// Equals compares this batch proof to another batch proof
func (bp *TrieBatchProof) Equals(o *TrieBatchProof) bool {
	if o == nil {
		return false
	}
	if len(bp.Proofs) != len(o.Proofs) {
		return false
	}
	for i, proof := range bp.Proofs {
		if !proof.Equals(o.Proofs[i]) {
			return false
		}
	}
	return true
}
