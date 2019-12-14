package blockProducerEvents

import "github.com/dapperlabs/flow-go/engine/consensus/modules/def"

// Processor consumes events produced by reactor.core
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnProducedBlock(*def.Block)
}

type ProducedBlockConsumer interface {
	OnProducedBlock(*def.Block)
}
