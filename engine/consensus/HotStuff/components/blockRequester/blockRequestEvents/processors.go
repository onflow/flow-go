package blockRequestEvents

// Processor consumes events produced by reactor.core
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnBlockRequestSent(blockMRH []byte)
}

type BlockRequestSentConsumer interface {
	OnBlockRequestSent(blockMRH []byte)
}
