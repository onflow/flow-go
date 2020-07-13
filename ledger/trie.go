package ledger

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

// TrieRead captures a trie read query
type TrieRead struct {
	RootHash RootHash
	Paths    []Path
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
type RootHash []byte

func (rh RootHash) String() string {
	return hex.EncodeToString(rh)
}

// Equals compares the root hash to another one
func (rh RootHash) Equals(o RootHash) bool {
	if o == nil {
		return false
	}
	return bytes.Equal(rh, o)
}

// Path captures storage path of a payload;
// where we store a payload in the ledger
type Path []byte

func (p Path) String() string {
	str := ""
	for _, i := range p {
		str += fmt.Sprintf("%08b", i)
	}
	if len(str) > 16 {
		str = str[0:8] + "..." + str[len(str)-8:]
	}
	return str
}

// Size returns the size of the path
func (p *Path) Size() int {
	return len(*p)
}

// Equals compares this path to another path
func (p Path) Equals(o Path) bool {
	if o == nil {
		return false
	}
	return bytes.Equal([]byte(p), []byte(o))
}

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	Key   Key
	Value Value
}

// Size returns the size of the payload
func (p *Payload) Size() int {
	return p.Key.Size() + p.Value.Size()
}

// IsEmpty returns true if key or value is not empty
func (p *Payload) IsEmpty() bool {
	return p.Size() == 0
}

// TODO fix me
func (p *Payload) String() string {
	// TODO improve this key, values
	return p.Key.String() + " " + p.Value.String()
}

// Equals compares this payload to another payload
func (p *Payload) Equals(other *Payload) bool {
	if other == nil {
		return false
	}
	if !p.Key.Equals(&other.Key) {
		return false
	}
	if !p.Value.Equals(other.Value) {
		return false
	}
	return true
}

// NewPayload returns a new payload
func NewPayload(key Key, value Value) *Payload {
	return &Payload{Key: key, Value: value}
}

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}

// TrieProof includes all the information needed to walk
// through a trie branch from an specific leaf node (key)
// up to the root of the trie.
type TrieProof struct {
	Path      Path     // path
	Payload   *Payload // payload
	Interims  [][]byte // the non-default intermediate nodes in the proof
	Inclusion bool     // flag indicating if this is an inclusion or exclusion
	Flags     []byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8    // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewTrieProof creates a new instance of Trie Proof
func NewTrieProof() *TrieProof {
	return &TrieProof{
		Path:      make([]byte, 0),
		Payload:   EmptyPayload(),
		Interims:  make([][]byte, 0),
		Inclusion: false,
		Flags:     make([]byte, 0),
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
		proofStr += fmt.Sprint("\t inclusion proof:\n")
	} else {
		proofStr += fmt.Sprint("\t noninclusion proof:\n")
	}
	interimIndex := 0
	for j := 0; j < int(p.Steps); j++ {
		// if bit is set
		if p.Flags[j/8]&(1<<int(7-j%8)) != 0 {
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
		if !bytes.Equal(inter, o.Interims[i]) {
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
	bp := NewTrieBatchProof()
	for i := 0; i < numberOfProofs; i++ {
		bp.AppendProof(NewTrieProof())
	}
	return bp
}

// Size returns the number of proofs
func (bp *TrieBatchProof) Size() int {
	return len(bp.Proofs)
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
