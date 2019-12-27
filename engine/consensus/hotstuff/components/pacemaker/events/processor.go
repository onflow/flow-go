package events

// Processor consumes events produced by reactor.pacemaker
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnForkChoiceTrigger(uint64)
	OnEnteringView(uint64)
	OnPassiveTillView(uint64)
}

// GenerateForkChoiceTiggerConsumer consumes events of type `OnForkChoiceTrigger`
// which are produced by pacemaker. An `OnGenerateForkChoiceTrigger` is emitted
// by the Pacemaker to trigger the eventual formation of a block for the view `viewNumber`
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ForkChoiceTriggerConsumer interface {
	OnForkChoiceTrigger(viewNumber uint64)
}

// EnteringViewConsumer consumes events of type `OnEnteringView`,
// which are produced by pacemaker whenever it enters the view `viewNumber`.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type EnteringViewConsumer interface {
	OnEnteringView(viewNumber uint64)
}

// PassiveTillViewConsumer events of type `OnPassiveTillView`, which are produced by pacemaker.
// It indicates that all active consensus participation should be suspended
// until (and including) `viewNumber`.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type PassiveTillViewConsumer interface {
	OnPassiveTillView(viewNumber uint64)
}
