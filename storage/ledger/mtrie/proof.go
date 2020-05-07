package mtrie

import (
	"fmt"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

// BatchProof is a struct that holds the proofs for several keys
// TODO (add key values to batch proof and make it self-included),
// so there is no need for two calls (read, proofs)
//  keys       [][][]byte // keys in this
//	values     [][][]byte // values
type BatchProof struct {
	values     [][][]byte // the non-default intermediate nodes in the proof
	inclusions []bool     // flag indicating if this is an inclusion or exclusion
	flags      [][]byte   // The flags of the proofs (is set if an intermediate node has a non-default)
	steps      []uint8    // number of steps for the proof (path len)
}

// newBatchProof creates a new instance of BatchProof
func NewBatchProof(flags [][]byte, values [][][]byte, inclusions []bool, steps []uint8) *BatchProof {
	holder := new(BatchProof)
	holder.flags = flags
	holder.values = values
	holder.inclusions = inclusions
	holder.steps = steps

	return holder
}

// GetSize returns number of proofs
func (bp *BatchProof) Size() int {
	return len(bp.flags)
}

func (bp *BatchProof) String() string {
	res := fmt.Sprintf("> proof holder includes %d proofs\n", bp.Size())
	for i, step := range bp.steps {
		flags := bp.flags[i]
		values := bp.values[i]
		flagStr := ""
		for _, f := range flags {
			flagStr += fmt.Sprintf("%08b", f)
		}
		proofStr := fmt.Sprintf("size: %d flags: %v\n", step, flagStr)
		if bp.inclusions[i] {
			proofStr += fmt.Sprintf("\t proof %d (inclusion)\n", i)
		} else {
			proofStr += fmt.Sprintf("\t proof %d (noninclusion)\n", i)
		}
		valueIndex := 0
		for j := 0; j < int(step); j++ {
			if utils.IsBitSet(flags, j) {
				proofStr += fmt.Sprintf("\t\t %d: [%x]\n", j, values[valueIndex])
				valueIndex++
			}
		}
		res = res + "\n" + proofStr
	}
	return res
}

// ExportProof return the flag, proofs, inclusion, an size of the proof at index i
func (bp *BatchProof) ExportProof(index int) ([]byte, [][]byte, bool, uint8) {
	return bp.flags[index], bp.values[index], bp.inclusions[index], bp.steps[index]
}

// EncodeProof encodes a proof holder into an array of byte arrays
// The following code is the encoding logic
// Each slice in the proofHolder is stored as a byte array, and the whole thing is stored
// as a [][]byte
// First we have a byte, and set the first bit to 1 if it is an inclusion proof
// Then the size is encoded as a single byte
// Then the flag is encoded (size is defined by size)
// Finally the proofs are encoded one at a time, and is stored as a byte array
func EncodeProof(bp *BatchProof) [][]byte {
	proofs := make([][]byte, 0)
	for i := 0; i < bp.Size(); i++ {
		flag, singleProof, inclusion, size := bp.ExportProof(i)
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
func DecodeProof(proofs [][]byte) (*BatchProof, error) {
	flags := make([][]byte, 0)
	values := make([][][]byte, 0)
	inclusions := make([]bool, 0)
	steps := make([]uint8, 0)
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
		step := proof[1:2]
		steps = append(steps, step...)
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
		values = append(values, byteProofs)
	}
	return NewBatchProof(flags, values, inclusions, steps), nil
}
