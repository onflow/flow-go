package proof

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// Proof includes all the information needed to walk
// through a trie branch from an specific leaf node (key)
// up to the root of the trie.
type Proof struct {
	Values    [][]byte // the non-default intermediate nodes in the proof
	Inclusion bool     // flag indicating if this is an inclusion or exclusion
	Flags     []byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	Steps     uint8    // number of steps for the proof (path len) // TODO: should this be a type allowing for larger values?
}

// NewProof creates a new instance of Proof
func NewProof() *Proof {
	return &Proof{
		Values:    make([][]byte, 0),
		Inclusion: false,
		Flags:     make([]byte, 0),
		Steps:     0,
	}
}

// Verify verifies the proof, by constructing all the
// hash from the leaf to the root and comparing the rootHash
// TODO RAMTIN
func (p *Proof) Verify(key []byte, value []byte, expectedRootHash []byte, expectedKeySize int) bool {
	path := common.KeyToPath(key)
	treeHeight := 8 * expectedKeySize
	leafHeight := treeHeight - int(p.Steps)             // p.Steps is the number of edges we are traversing until we hit the compactified leaf.
	if !(0 <= leafHeight && leafHeight <= treeHeight) { // sanity check
		return false
	}
	// We start with the leaf and hash our way upwards towards the root
	proofIndex := len(p.Values) - 1                                      // the index of the last non-default value furthest down the tree (-1 if there is none)
	computed := common.ComputeCompactValue(path, key, value, leafHeight) // we first compute the hash of the fully-expanded leaf (at height 0)
	for h := leafHeight + 1; h <= treeHeight; h++ {                      // then, we hash our way upwards until we hit the root (at height `treeHeight`)
		// we are currently at a node n (initially the leaf). In this iteration, we want to compute the
		// parent's hash. Here, h is the height of the parent, whose hash want to compute.
		// The parent has two children: child n, whose hash we have already computed (aka `computed`);
		// and the sibling to node n, whose hash (aka `siblingHash`) must be defined by the Proof.

		var siblingHash []byte
		if utils.IsBitSet(p.Flags, treeHeight-h) { // if flag is set, siblingHash is stored in the proof
			if proofIndex < 0 { // proof invalid: too few values
				return false
			}
			siblingHash = p.Values[proofIndex]
			proofIndex--
		} else { // otherwise, siblingHash is a default hash
			siblingHash = common.GetDefaultHashForHeight(h - 1)
		}
		// hashing is order dependant
		if utils.IsBitSet(path, treeHeight-h) { // we hash our way up to the parent along the parent's right branch
			computed = common.HashInterNode(siblingHash, computed)
		} else { // we hash our way up to the parent along the parent's left branch
			computed = common.HashInterNode(computed, siblingHash)
		}
	}
	return bytes.Equal(computed, expectedRootHash) == p.Inclusion
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
	valueIndex := 0
	for j := 0; j < int(p.Steps); j++ {
		if utils.IsBitSet(p.Flags, j) {
			proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, p.Values[valueIndex])
			valueIndex++
		}
	}
	return proofStr
}

// Export return the flag, proofs, inclusion, an size of the proof
func (p *Proof) Export() ([]byte, [][]byte, bool, uint8) {
	return p.Flags, p.Values, p.Inclusion, p.Steps
}

// BatchProof is a struct that holds the proofs for several keys
//
// TODO (add key values to batch proof and make it self-included),
// so there is no need for two calls (read, proofs)
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

// Verify verifies all the proof inside the batchproof
func (bp *BatchProof) Verify(keys [][]byte, values [][]byte, expectedRootHash []byte, expectedKeySize int) bool {
	for i, p := range bp.Proofs {
		// any invalid proof
		if !p.Verify(keys[i], values[i], expectedRootHash, expectedKeySize) {
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

// EncodeBatchProof encodes all the content of a batch proof into an array of byte arrays
func EncodeBatchProof(bp *BatchProof) ([][]byte, int) {
	proofs := make([][]byte, 0)
	totalLength := 0
	// for each proof we create a byte array
	for _, p := range bp.Proofs {
		flag, values, inclusion, steps := p.Export()

		// 1. set the first bit to 1 if it is an inclusion proof
		byteInclusion := make([]byte, 1)
		if inclusion {
			utils.SetBit(byteInclusion, 0)
		}
		// 2. steps is encoded as a single byte
		byteSteps := []byte{steps}
		proof := append(byteInclusion, byteSteps...)

		// 3. include flag size first and then all the flags
		flagSize := []byte{uint8(len(flag))}
		proof = append(proof, flagSize...)
		proof = append(proof, flag...)

		// 4. and finally include all the hash values
		for _, v := range values {
			proof = append(proof, v...)
		}
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
		pInst.Inclusion = utils.IsBitSet(byteInclusion, 0)
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
		pInst.Values = byteProofs
		bp.AppendProof(pInst)
	}
	return bp, nil
}

// SplitProofsByPath splits a set of unordered path and proof pairs based on the value of bit (bitIndex) of path
func SplitProofsByPath(paths [][]byte, proofs []*Proof, bitIndex int) ([][]byte, []*Proof, [][]byte, []*Proof, error) {
	rpaths := make([][]byte, 0, len(paths))
	rproofs := make([]*Proof, 0, len(proofs))
	lpaths := make([][]byte, 0, len(paths))
	lproofs := make([]*Proof, 0, len(proofs))

	for i, path := range paths {
		bitIsSet, err := common.IsBitSet(path, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split key proof pairs , error: %v", err)
		}
		if bitIsSet {
			rpaths = append(rpaths, path)
			rproofs = append(rproofs, proofs[i])
		} else {
			lpaths = append(lpaths, path)
			lproofs = append(lproofs, proofs[i])
		}
	}
	return lpaths, lproofs, rpaths, rproofs, nil
}
