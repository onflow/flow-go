package def

import (
	"bytes"
	"crypto/sha256"

	"github.com/dapperlabs/flow-go/engine/consensus/modules/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/modules/utils"
)

// NOTE: renamed QC to QuorumCertificate for consistency
type QuorumCertificate struct {
	View     uint64
	BlockMRH []byte
	*crypto.AggregatedSignature
	// Hash of QC, calculated and stored at construction
	Hash []byte
}

func NewQC(blockMRH []byte, sigs []*crypto.Signature, signersBitfieldLength uint32) *QuorumCertificate {
	qc := &QuorumCertificate{
		BlockMRH:            blockMRH,
		AggregatedSignature: crypto.AggregateSigs(sigs, signersBitfieldLength),
	}
	qc.ComputeHash()

	return qc
}

// TODO: merge changes from mocking signature
func AggregateSignature(sigs []*crypto.Signature) *crypto.AggregatedSignature {
	rawSigs := []byte{}
	signers := []bool{}

	return &crypto.AggregatedSignature{
		RawSignature: rawSigs,
		Signers:      signers,
	}
}

// compute and set hash of QC
func (qc *QuorumCertificate) ComputeHash() {
	data := bytes.Join(
		// TODO: viewNumber isn't included here?
		[][]byte{
			utils.ConvertUintToByte(qc.View),
			qc.BlockMRH,
			// TODO: replace with hash of crypto.AggregatedSignature
			qc.AggregatedSignature.RawSignature,
			utils.ConvertBoolSliceToByteSlice(qc.AggregatedSignature.Signers),
		},
		[]byte{},
	)

	qcHash := sha256.Sum256(data)

	qc.Hash = qcHash[:]
}

// Verify QC and return true iff QC is valid
func (qc *QuorumCertificate) IsValid() bool {
	// TODO implement QuorumCertificate.IsValid():
	// * check signature is valid
	// * verify that QC is signed by _more that_ 2/3 of stake
	return true
}
