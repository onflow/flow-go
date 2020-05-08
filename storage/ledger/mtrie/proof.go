package mtrie

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

type Proof struct {
	values    [][]byte // the non-default intermediate nodes in the proof
	inclusion bool     // flag indicating if this is an inclusion or exclusion
	flags     []byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	steps     uint8    // number of steps for the proof (path len)
}

// NewProof creates a new instance of Proof
func NewProof() *Proof {
	p := new(Proof)
	p.values = make([][]byte, 0)
	p.inclusion = false
	p.flags = make([]byte, 0)
	p.steps = 0
	return p
}

// Verify verifies the proof
func (p *Proof) Verify(key []byte, value []byte, rootHash []byte, trieMaxHeight int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(p.values) != 0 {
		proofIndex = len(p.values) - 1
	}
	// base case at the bottom of the trie
	computed := ComputeCompactValue(key, value, trieMaxHeight-int(p.steps)-1, trieMaxHeight)
	for i := int(p.steps) - 1; i > -1; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i) {
			if !utils.IsBitSet(p.flags, i) {
				computed = HashInterNode(GetDefaultHashForHeight((trieMaxHeight-i)-2), computed)
			} else {
				computed = HashInterNode(p.values[proofIndex], computed)
				proofIndex--
			}
		} else {
			if !utils.IsBitSet(p.flags, i) {
				computed = HashInterNode(computed, GetDefaultHashForHeight((trieMaxHeight-i)-2))
			} else {
				computed = HashInterNode(computed, p.values[proofIndex])
				proofIndex--
			}
		}
	}
	return bytes.Equal(computed, rootHash) == p.inclusion
}

func (p *Proof) String() string {
	flagStr := ""
	for _, f := range p.flags {
		flagStr += fmt.Sprintf("%08b", f)
	}
	proofStr := fmt.Sprintf("size: %d flags: %v\n", p.steps, flagStr)
	if p.inclusion {
		proofStr += fmt.Sprint("\t inclusion proof:\n")
	} else {
		proofStr += fmt.Sprint("\t noninclusion proof:\n")
	}
	valueIndex := 0
	for j := 0; j < int(p.steps); j++ {
		if utils.IsBitSet(p.flags, j) {
			proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, p.values[valueIndex])
			valueIndex++
		}
	}
	return proofStr
}

// Export return the flag, proofs, inclusion, an size of the proof
func (p *Proof) Export() ([]byte, [][]byte, bool, uint8) {
	return p.flags, p.values, p.inclusion, p.steps
}

// BatchProof is a struct that holds the proofs for several keys
// TODO (add key values to batch proof and make it self-included),
// so there is no need for two calls (read, proofs)
//  keys       [][][]byte // keys in this
//	values     [][][]byte // values
type BatchProof struct {
	Proofs []*Proof
}

// NewBatchProof creates a new instance of BatchProof
func NewBatchProof() *BatchProof {
	bp := new(BatchProof)
	bp.Proofs = make([]*Proof, 0)
	return bp
}

func NewBatchProofWithEmptyProofs(numberOfProofs int) *BatchProof {
	bp := NewBatchProof()
	for i := 0; i < numberOfProofs; i++ {
		bp.AppendProof(NewProof())
	}
	return bp
}

// GetSize returns number of proofs
func (bp *BatchProof) Size() int {
	return len(bp.Proofs)
}

// Verify verifies all the proof inside the batchproof
func (bp *BatchProof) Verify(keys [][]byte, values [][]byte, rootHash []byte, trieMaxHeight int) bool {
	for i, p := range bp.Proofs {
		// any invalid proof
		if !p.Verify(keys[i], values[i], rootHash, trieMaxHeight) {
			return false
		}
	}
	return true
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

// EncodeProof encodes a proof holder into an array of byte arrays
// The following code is the encoding logic
// Each slice in the proofHolder is stored as a byte array, and the whole thing is stored
// as a [][]byte
// First we have a byte, and set the first bit to 1 if it is an inclusion proof
// Then the size is encoded as a single byte
// Then the flag is encoded (size is defined by size)
// Finally the proofs are encoded one at a time, and is stored as a byte array
func EncodeBatchProof(bp *BatchProof) [][]byte {
	proofs := make([][]byte, 0)
	for _, p := range bp.Proofs {
		flag, singleProof, inclusion, size := p.Export()
		byteSize := []byte{size}
		byteInclusion := make([]byte, 1)
		if inclusion {
			utils.SetBit(byteInclusion, 0)
		}
		proof := append(byteInclusion, byteSize...)

		flagSize := []byte{uint8(len(flag))}
		proof = append(proof, flagSize...)
		proof = append(proof, flag...)

		for _, p := range singleProof {
			proof = append(proof, p...)
		}
		// ledgerStorage is a struct that holds our SM
		proofs = append(proofs, proof)
	}
	return proofs
}

// DecodeProof takes in an encodes array of byte arrays an converts them into a proofHolder
func DecodeBatchProof(proofs [][]byte) (*BatchProof, error) {
	bp := NewBatchProof()
	// The decode logic is as follows:
	// The first byte in the array is the inclusion flag, with the first bit set as the inclusion (1 = inclusion, 0 = non-inclusion)
	// The second byte is size, needs to be converted to uint8
	// The next 32 bytes are the flag
	// Each subsequent 32 bytes are the proofs needed for the verifier
	// Each result is put into their own array and put into a proofHolder
	for _, proof := range proofs {
		if len(proof) < 4 {
			return nil, fmt.Errorf("error decoding the proof: proof size too small")
		}
		pInst := NewProof()
		byteInclusion := proof[0:1]
		pInst.inclusion = utils.IsBitSet(byteInclusion, 0)
		step := proof[1:2]
		pInst.steps = step[0]
		flagSize := int(proof[2])
		if flagSize < 1 {
			return nil, fmt.Errorf("error decoding the proof: flag size should be greater than 0")
		}
		pInst.flags = proof[3 : flagSize+3]
		byteProofs := make([][]byte, 0)
		for i := flagSize + 3; i < len(proof); i += 32 {
			// TODO understand the logic here
			if i+32 <= len(proof) {
				byteProofs = append(byteProofs, proof[i:i+32])
			} else {
				byteProofs = append(byteProofs, proof[i:])
			}
		}
		pInst.values = byteProofs
		bp.AppendProof(pInst)
	}
	return bp, nil
}
