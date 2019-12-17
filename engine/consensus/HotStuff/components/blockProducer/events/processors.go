package blockProducerEvents

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// Processor consumes events produced by BlockProducer
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnProducedBlock(*def.Block)
}

// ProducedBlockConsumer consumes the following type of event produced by BlockProducer:
// whenever a block is produced, the `OnProducedBlock` event is triggered
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type ProducedBlockConsumer interface {
	OnProducedBlock(*def.Block)
}
