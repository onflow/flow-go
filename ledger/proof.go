package ledger

import (
	"fmt"
)

// Proof includes all the information needed to walk
// through a trie branch from an specific leaf node (key)
// up to the root of the trie.
// TODO add Payload (Key, Value)
// TODO add version
type Proof struct {
	// Key       Key      // ledger key
	// Value     Value    // ledger value
	Interims  [][]byte // the non-default intermediate nodes in the proof
	Inclusion bool     // flag indicating if this is an inclusion or exclusion
	Flags     []byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8    // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewProof creates a new instance of Proof
func NewProof() *Proof {
	return &Proof{
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
	if p.Inclusion {
		proofStr += fmt.Sprint("\t inclusion proof:\n")
	} else {
		proofStr += fmt.Sprint("\t noninclusion proof:\n")
	}
	interimIndex := 0
	for j := 0; j < int(p.Steps); j++ {
		if isBitSet(p.Flags, j) {
			proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, p.Interims[interimIndex])
			interimIndex++
		}
	}
	return proofStr
}

// Encode encodes all the content of a  proof into a byte slice
func (p *Proof) Encode() []byte {
	// 1. first byte is reserved for inclusion flag
	byteInclusion := make([]byte, 1)
	if p.Inclusion {
		// set the first bit to 1 if it is an inclusion proof
		byteInclusion[0] |= 1 << 7
	}
	// 2. steps is encoded as a single byte
	byteSteps := []byte{p.Steps}
	proof := append(byteInclusion, byteSteps...)

	// 3. include flag size first and then all the flags
	flagSize := []byte{uint8(len(p.Flags))}
	proof = append(proof, flagSize...)
	proof = append(proof, p.Flags...)

	// 4. and finally include all interims (hash values)
	for _, inter := range p.Interims {
		proof = append(proof, inter...)
	}

	return proof
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

// Encode encodes all the content of a batch proof into a slice of byte slices and total len
// TODO change this to only an slice of bytes
func (bp *BatchProof) Encode() ([][]byte, int) {
	proofs := make([][]byte, 0)
	totalLength := 0
	// for each proof we create a byte array
	for _, p := range bp.Proofs {
		proof := p.Encode()
		totalLength += len(proof)
		proofs = append(proofs, proof)
	}
	return proofs, totalLength
}

// DecodeBatchProof takes in an encodes array of byte arrays an converts them into a BatchProof
func DecodeBatchProof(proofs [][]byte) (*BatchProof, error) {
	bp := NewBatchProof()
	// The decode logic is as follows:
	// The first byte in the array is the inclusion flag, with the first bit set as the inclusion (1 = inclusion, 0 = non-inclusion)
	// The second byte is size, needs to be converted to uint8
	// The next 32 bytes are the flag
	// Each subsequent 32 bytes are the proofs needed for the verifier
	// Each result is put into their own array and put into a BatchProof
	for _, proof := range proofs {
		if len(proof) < 4 {
			return nil, fmt.Errorf("error decoding the proof: proof size too small")
		}
		pInst := NewProof()
		byteInclusion := proof[0:1]
		pInst.Inclusion = isBitSet(byteInclusion, 0)
		step := proof[1:2]
		pInst.Steps = step[0]
		flagSize := int(proof[2])
		if flagSize < 1 {
			return nil, fmt.Errorf("error decoding the proof: flag size should be greater than 0")
		}
		pInst.Flags = proof[3 : flagSize+3]
		byteProofs := make([][]byte, 0)
		for i := flagSize + 3; i < len(proof); i += 32 {
			// TODO understand the logic here
			if i+32 <= len(proof) {
				byteProofs = append(byteProofs, proof[i:i+32])
			} else {
				byteProofs = append(byteProofs, proof[i:])
			}
		}
		pInst.Interims = byteProofs
		bp.AppendProof(pInst)
	}
	return bp, nil
}

func isBitSet(b []byte, i int) bool {
	return b[i/8]&(1<<int(7-i%8)) != 0
}
