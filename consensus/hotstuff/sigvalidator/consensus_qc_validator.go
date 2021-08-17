package sigvalidator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusQCValidator struct {
	packer hotstuff.Packer // to decode the byte slices of the signature data into QC fields
}

func NewConsensusQCValidator() *ConsensusQCValidator {
	return &ConsensusQCValidator{
		packer: nil,
	}
}

func (v *ConsensusQCValidator) ValidateQC(qc *flow.QuorumCertificate) error {
	panic("TO IMPLEMENT")
}
