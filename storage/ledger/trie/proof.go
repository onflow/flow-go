package trie

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// proofHolder is a struct that holds the proofs and flags from a proof check
type proofHolder struct {
	flags      [][]byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	proofs     [][][]byte // the non-default nodes in the proof
	inclusions []bool     // flag indicating if this is an inclusion or exclusion
	sizes      []uint8    // size of the proof in steps
}

// newProofHolder is a constructor for proofHolder
func newProofHolder(flags [][]byte, proofs [][][]byte, inclusions []bool, sizes []uint8) *proofHolder {
	holder := new(proofHolder)
	holder.flags = flags
	holder.proofs = proofs
	holder.inclusions = inclusions
	holder.sizes = sizes

	return holder
}

// GetSize returns the length of the proofHolder
func (p *proofHolder) GetSize() int {
	return len(p.flags)
}

// ExportProof return the flag, proofs, inclusion, an size of the proof at index i
func (p *proofHolder) ExportProof(index int) ([]byte, [][]byte, bool, uint8) {
	return p.flags[index], p.proofs[index], p.inclusions[index], p.sizes[index]
}

// ExportWholeProof returns the proof holder seperated into it's individual fields
func (p *proofHolder) ExportWholeProof() ([][]byte, [][][]byte, []bool, []uint8) {
	return p.flags, p.proofs, p.inclusions, p.sizes
}

func (p proofHolder) String() string {
	res := fmt.Sprintf("> proof holder includes %d proofs\n", len(p.sizes))
	for i, size := range p.sizes {
		flags := p.flags[i]
		proof := p.proofs[i]
		flagStr := ""
		for _, f := range flags {
			flagStr += fmt.Sprintf("%08b", f)
		}
		proofStr := fmt.Sprintf("size: %d flags: %v\n", size, flagStr)
		if p.inclusions[i] {
			proofStr += fmt.Sprintf("\t proof %d (inclusion)\n", i)
		} else {
			proofStr += fmt.Sprintf("\t proof %d (noninclusion)\n", i)
		}
		proofIndex := 0
		for j := 0; j < int(size); j++ {
			if utils.IsBitSet(flags, j) {
				proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, proof[proofIndex])
				proofIndex++
			}
		}
		res = res + "\n" + proofStr
	}
	return res
}

// VerifyInclusionProof calculates the inclusion proof from a given rootNode, flag, proof list, and size.
//
// This function is exclusively for inclusive proofs
func VerifyInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}
	// base case at the bottom of the trie
	computed := ComputeCompactValue(key, value, height-int(size)-1, height)
	for i := int(size) - 1; i > -1; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i) {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(GetDefaultHashForHeight((height-i)-2), computed)
			} else {
				computed = HashInterNode(proof[proofIndex], computed)
				proofIndex--
			}
		} else {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(computed, GetDefaultHashForHeight((height-i)-2))
			} else {
				computed = HashInterNode(computed, proof[proofIndex])
				proofIndex--
			}
		}
	}
	return bytes.Equal(computed, root)
}

func VerifyNonInclusionProof(key []byte, value []byte, flag []byte, proof [][]byte, size uint8, root []byte, height int) bool {
	// get index of proof we start our calculations from
	proofIndex := 0

	if len(proof) != 0 {
		proofIndex = len(proof) - 1
	}
	// base case at the bottom of the trie
	computed := ComputeCompactValue(key, value, height-int(size)-1, height)
	for i := int(size) - 1; i > -1; i-- {
		// hashing is order dependant
		if utils.IsBitSet(key, i) {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(GetDefaultHashForHeight((height-i)-2), computed)
			} else {
				computed = HashInterNode(proof[proofIndex], computed)
				proofIndex--
			}
		} else {
			if !utils.IsBitSet(flag, i) {
				computed = HashInterNode(computed, GetDefaultHashForHeight((height-i)-2))
			} else {
				computed = HashInterNode(computed, proof[proofIndex])
				proofIndex--
			}
		}
	}
	return !bytes.Equal(computed, root)
}

// EncodeProof encodes a proof holder into an array of byte arrays
// The following code is the encoding logic
// Each slice in the proofHolder is stored as a byte array, and the whole thing is stored
// as a [][]byte
// First we have a byte, and set the first bit to 1 if it is an inclusion proof
// Then the size is encoded as a single byte
// Then the flag is encoded (size is defined by size)
// Finally the proofs are encoded one at a time, and is stored as a byte array
func EncodeProof(pholder *proofHolder) [][]byte {
	proofs := make([][]byte, 0)
	for i := 0; i < pholder.GetSize(); i++ {
		flag, singleProof, inclusion, size := pholder.ExportProof(i)
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
func DecodeProof(proofs [][]byte) (*proofHolder, error) {
	flags := make([][]byte, 0)
	newProofs := make([][][]byte, 0)
	inclusions := make([]bool, 0)
	sizes := make([]uint8, 0)
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
		byteInclusion := proof[0:1]
		inclusion := utils.IsBitSet(byteInclusion, 0)
		inclusions = append(inclusions, inclusion)
		size := proof[1:2]
		sizes = append(sizes, size...)
		flagSize := int(proof[2])
		if flagSize < 1 {
			return nil, fmt.Errorf("error decoding the proof: flag size should be greater than 0")
		}
		flags = append(flags, proof[3:flagSize+3])
		byteProofs := make([][]byte, 0)
		for i := flagSize + 3; i < len(proof); i += 32 {
			// TODO understand the logic here
			if i+32 <= len(proof) {
				byteProofs = append(byteProofs, proof[i:i+32])
			} else {
				byteProofs = append(byteProofs, proof[i:])
			}
		}
		newProofs = append(newProofs, byteProofs)
	}
	return newProofHolder(flags, newProofs, inclusions, sizes), nil
}
