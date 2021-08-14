package sigvalidator

import (
	"github.com/onflow/flow-go/model/flow"
)

type SingleQCValidator struct {
}

func (v *SingleQCValidator) ValidateQC(qc *flow.QuorumCertificate) error {
	panic("TO IMPLEMENT")
}
