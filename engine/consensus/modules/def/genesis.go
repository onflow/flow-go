package def

import (
	"bytes"
	"crypto/sha256"

	"github.com/dapperlabs/flow-go/engine/consensus/modules/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/modules/utils"
)

func NewGenesisBlock() *Block {
	b := &Block{}

	b.BlockMRH = computeGenesisBlockHash()

	return b
}

func computeGenesisBlockHash() []byte {
	data := bytes.Join(
		[][]byte{
			// hash of View
			utils.ConvertUintToByte(0),
		},
		[]byte{},
	)
	blockMRH := sha256.Sum256(data)
	return blockMRH[:]

}

func NewGenesisQC(genesisBlockHash []byte) *QuorumCertificate {
	qc := &QuorumCertificate{
		View:     0,
		BlockMRH: genesisBlockHash,
		AggregatedSignature: &crypto.AggregatedSignature{
			RawSignature: []byte{},
			Signers:      []bool{},
		},
	}
	qc.ComputeHash()

	return qc
}
