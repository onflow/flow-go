package events

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
)

// Processor consumes events produced by forkchoice.ForkChoice
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnQcFromVotesIncorporated(*types.QuorumCertificate)

	// The `viewNumber` specifies the view to the block that should be built and `qc` the
	// quorum certificate that is supposed to be in the block.
	OnForkChoiceGenerated(viewNumber uint64, qc *types.QuorumCertificate)
}

// IncorporatedQuorumCertificateProcessor consumes the following type of event produced by forkchoice.ForkChoice
// whenever a QC is incorporated into the consensus state, the `OnQcFromVotesIncorporated` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type IncorporatedQuorumCertificateConsumer interface {
	OnQcFromVotesIncorporated(*types.QuorumCertificate)
}

// ForkChoiceProcessor consumes the following type of event produced by forkchoice.ForkChoice
// whenever a fork choice is generated, the `OnForkChoiceGenerated` event is triggered.
// The `viewNumber` specifies the view to the block that should be built and `qc` the
// quorum certificate that is supposed to be in the block.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ForkChoiceGeneratedConsumer interface {
	OnForkChoiceGenerated(viewNumber uint64, qc *types.QuorumCertificate)
}
