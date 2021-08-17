package hotstuff

import (
	"github.com/onflow/flow-go/model/flow"
)

type QCValidator interface {
	// ValidateQC validates the cryptographic signature of a QC.
	// It returns:
	//   - nil if the QC's signature is valid
	//   - model.InvalidQCError if the block's signature is invalid
	//   - other error if there is an exception
	ValidateQC(qc *flow.QuorumCertificate) error
}
