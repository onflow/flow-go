package sigvalidator

import (
	"github.com/onflow/flow-go/model/flow"
)

type CollectionQCValidator struct {
}

// ValidateQC validates the aggregated signature of the QuorumCertificate
// The validation is stateless
func (v *CollectionQCValidator) ValidateQC(qc *flow.QuorumCertificate) error {
	panic("TO IMPLEMENT")
}
