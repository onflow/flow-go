package events

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// Processor consumes events produced by forkchoice.ForkChoice
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnIncorporatedQuorumCertificate(*def.QuorumCertificate)
	OnForkChoiceGenerated(*def.QuorumCertificate)
}

// IncorporatedQuorumCertificateProcessor consumes the following type of event produced by forkchoice.ForkChoice
// whenever a QC is incorporated into the consensus state, the `OnIncorporatedQuorumCertificate` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type IncorporatedQuorumCertificateConsumer interface {
	OnIncorporatedQuorumCertificate(*def.QuorumCertificate)
}

// ForkChoiceProcessor consumes the following type of event produced by forkchoice.ForkChoice
// whenever a fork choice is generated, the `OnForkChoiceGenerated` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ForkChoiceConsumer interface {
	OnForkChoiceGenerated(*def.QuorumCertificate)
}
