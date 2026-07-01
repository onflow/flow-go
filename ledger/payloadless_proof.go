package ledger

import (
	"bytes"
	"fmt"

	"github.com/onflow/flow-go/ledger/common/hash"
)

// PayloadlessTrieProof includes all the information needed to walk
// through a payloadless trie branch from an specific leaf node (key)
// up to the root of the trie.
//
// Unlike [TrieProof], which stores the full Payload at the leaf, a
// PayloadlessTrieProof stores only the leaf hash (HashLeaf(path, value)),
// matching the storage of the payloadless trie. The caller is responsible
// for retrieving the original value separately if needed.
type PayloadlessTrieProof struct {
	Path      Path        // path
	LeafHash  *hash.Hash  // leaf hash HashLeaf(path, value); nil for empty leaves and non-inclusion proofs
	Interims  []hash.Hash // the non-default intermediate nodes in the proof
	Inclusion bool        // flag indicating if this is an inclusion or exclusion proof
	Flags     []byte      // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8       // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewPayloadlessTrieProof creates a new instance of PayloadlessTrieProof
func NewPayloadlessTrieProof() *PayloadlessTrieProof {
	return &PayloadlessTrieProof{
		LeafHash:  nil,
		Interims:  make([]hash.Hash, 0),
		Inclusion: false,
		Flags:     make([]byte, PathLen),
		Steps:     0,
	}
}

func (p *PayloadlessTrieProof) String() string {
	flagStr := ""
	for _, f := range p.Flags {
		flagStr += fmt.Sprintf("%08b", f)
	}
	proofStr := fmt.Sprintf("size: %d flags: %v\n", p.Steps, flagStr)
	leafHashStr := "nil"
	if p.LeafHash != nil {
		leafHashStr = fmt.Sprintf("%x", *p.LeafHash)
	}
	proofStr += fmt.Sprintf("\t path: %v leafHash: %s\n", p.Path, leafHashStr)

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

// Equals compares this proof to another payloadless proof
func (p *PayloadlessTrieProof) Equals(o *PayloadlessTrieProof) bool {
	if o == nil {
		return false
	}
	if !p.Path.Equals(o.Path) {
		return false
	}
	if (p.LeafHash == nil) != (o.LeafHash == nil) {
		return false
	}
	if p.LeafHash != nil && *p.LeafHash != *o.LeafHash {
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

// PayloadlessTrieBatchProof is a struct that holds the payloadless proofs for several keys
//
// so there is no need for two calls (read, proofs)
type PayloadlessTrieBatchProof struct {
	Proofs []*PayloadlessTrieProof
}

// NewPayloadlessTrieBatchProof creates a new instance of PayloadlessTrieBatchProof
func NewPayloadlessTrieBatchProof() *PayloadlessTrieBatchProof {
	bp := new(PayloadlessTrieBatchProof)
	bp.Proofs = make([]*PayloadlessTrieProof, 0)
	return bp
}

// NewPayloadlessTrieBatchProofWithEmptyProofs creates an instance of PayloadlessTrieBatchProof
// filled with n newly created proofs (empty)
func NewPayloadlessTrieBatchProofWithEmptyProofs(numberOfProofs int) *PayloadlessTrieBatchProof {
	bp := new(PayloadlessTrieBatchProof)
	bp.Proofs = make([]*PayloadlessTrieProof, numberOfProofs)
	for i := range numberOfProofs {
		bp.Proofs[i] = NewPayloadlessTrieProof()
	}
	return bp
}

// Size returns the number of proofs
func (bp *PayloadlessTrieBatchProof) Size() int {
	return len(bp.Proofs)
}

// Paths returns the slice of paths for this batch proof
func (bp *PayloadlessTrieBatchProof) Paths() []Path {
	paths := make([]Path, len(bp.Proofs))
	for i, p := range bp.Proofs {
		paths[i] = p.Path
	}
	return paths
}

// LeafHashes returns the slice of leaf hashes for this batch proof
func (bp *PayloadlessTrieBatchProof) LeafHashes() []*hash.Hash {
	leafHashes := make([]*hash.Hash, len(bp.Proofs))
	for i, p := range bp.Proofs {
		leafHashes[i] = p.LeafHash
	}
	return leafHashes
}

func (bp *PayloadlessTrieBatchProof) String() string {
	res := fmt.Sprintf("payloadless trie batch proof includes %d proofs: \n", bp.Size())
	for _, proof := range bp.Proofs {
		res = res + "\n" + proof.String()
	}
	return res
}

// AppendProof adds a proof to the batch proof
func (bp *PayloadlessTrieBatchProof) AppendProof(p *PayloadlessTrieProof) {
	bp.Proofs = append(bp.Proofs, p)
}

// MergeInto adds all of its proofs into the dest batch proof
func (bp *PayloadlessTrieBatchProof) MergeInto(dest *PayloadlessTrieBatchProof) {
	for _, p := range bp.Proofs {
		dest.AppendProof(p)
	}
}

// Equals compares this batch proof to another batch proof
func (bp *PayloadlessTrieBatchProof) Equals(o *PayloadlessTrieBatchProof) bool {
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
