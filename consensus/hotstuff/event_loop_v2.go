package hotstuff

import "github.com/onflow/flow-go/model/flow"

// EventLoopV2 performs buffer and processing of incoming proposals and QCs.
type EventLoopV2 interface {
	// SubmitProposal accepts proposal for processing. Proposal will be dispatched on worker thread.
	SubmitProposal(proposalHeader *flow.Header, parentView uint64)

	// SubmitTrustedQC accepts QC for processing. QC will be dispatched on worker thread.
	// CAUTION: QC is trusted (_not_ validated again)
	SubmitTrustedQC(qc *flow.QuorumCertificate)
}
