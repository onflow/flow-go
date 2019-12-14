package events

// Processor consumes events produced by reactor.pacemaker
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	BlockProposalTrigger(uint64)
	EnteringView(uint64)
	EnteringCatchupMode(uint64)
	EnteringVotingMode(uint64)
}

// BlockProposalTriggerConsumer events of type `OnBlockProposalTrigger`
// which are produced by reactor.core.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type BlockProposalTriggerConsumer interface {
	OnBlockProposalTrigger(uint64)
}

// EnteringViewConsumer events of type `OnEnteringView`
// which are produced by reactor.core.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type EnteringViewConsumer interface {
	OnEnteringView(uint64)
}

// EnteringCatchupModeConsumer events of type `OnEnteringCatchupMode`
// which are produced by reactor.core.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type EnteringCatchupModeConsumer interface {
	OnEnteringCatchupMode(uint64)
}

// EnteringVotingModeConsumer events of type `OnEnteringVotingMode`
// which are produced by reactor.core.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type EnteringVotingModeConsumer interface {
	OnEnteringVotingMode(uint64)
}
