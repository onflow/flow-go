package def

import (
	"bytes"
	"crypto/sha256"

	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/utils"
)

// TODO: merge changes from mocking signature
func AggregateSignature(sigs []*crypto.Signature) *crypto.AggregatedSignature {
	rawSigs := []byte{}
	signers := []bool{}

	return &crypto.AggregatedSignature{
		RawSignature: rawSigs,
		Signers:      signers,
	}
}

// Verify QC and return true iff QC is valid
func (qc *QuorumCertificate) IsValid() bool {
	// TODO implement QuorumCertificate.IsValid():
	// * check signature is valid
	// * verify that QC is signed by _more that_ 2/3 of stake
	return true
}
