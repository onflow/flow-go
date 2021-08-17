package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/model/flow"
)

type CombinedQCValidator struct {
	packer *signature.ConsensusSigPackerImpl // to decode the byte slices of the signature data into QC fields
}

func NewCombinedQCValidator() *CombinedQCValidator {
	return &CombinedQCValidator{
		packer: nil,
	}
}

func (v *CombinedQCValidator) ValidateQC(qc *flow.QuorumCertificate) error {
	panic("TO IMPLEMENT")
}
