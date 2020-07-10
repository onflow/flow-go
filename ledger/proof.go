package ledger

import (
	"fmt"
)

// Proof includes all the information needed to walk
// through a trie branch from an specific leaf node (key)
// up to the root of the trie.
type Proof struct {
	Path      Path     // path
	Payload   Payload  // payload
	Interims  [][]byte // the non-default intermediate nodes in the proof
	Inclusion bool     // flag indicating if this is an inclusion or exclusion
	Flags     []byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8    // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewProof creates a new instance of Proof
func NewProof() *Proof {
	return &Proof{
		Path:      make([]byte, 0),
		Payload:   *EmptyPayload(),
		Interims:  make([][]byte, 0),
		Inclusion: false,
		Flags:     make([]byte, 0),
		Steps:     0,
	}
}

func (p *Proof) String() string {
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

// BatchProof is a struct that holds the proofs for several keys
//
// TODO (add key interims to batch proof and make it self-included),
// so there is no need for two calls (read, proofs)
// TODO add version
type BatchProof struct {
	Proofs []*Proof
}

// NewBatchProof creates a new instance of BatchProof
func NewBatchProof() *BatchProof {
	bp := new(BatchProof)
	bp.Proofs = make([]*Proof, 0)
	return bp
}

// NewBatchProofWithEmptyProofs creates an instance of Batchproof
// filled with n newly created proofs (empty)
func NewBatchProofWithEmptyProofs(numberOfProofs int) *BatchProof {
	bp := NewBatchProof()
	for i := 0; i < numberOfProofs; i++ {
		bp.AppendProof(NewProof())
	}
	return bp
}

// Size returns the number of proofs
func (bp *BatchProof) Size() int {
	return len(bp.Proofs)
}

func (bp *BatchProof) String() string {
	res := fmt.Sprintf("batch proof includes %d proofs: \n", bp.Size())
	for _, proof := range bp.Proofs {
		res = res + "\n" + proof.String()
	}
	return res
}

// AppendProof adds a proof to the batch proof
func (bp *BatchProof) AppendProof(p *Proof) {
	bp.Proofs = append(bp.Proofs, p)
}

// MergeInto adds all of its proofs into the dest batch proof
func (bp *BatchProof) MergeInto(dest *BatchProof) {
	for _, p := range bp.Proofs {
		dest.AppendProof(p)
	}
}
