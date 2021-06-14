package fetcher

import (
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/module"
)

// AssignedChunkProcessor should be implemented by the verification node engine responsible
// for processing assigned chunk locators to this node.
//
// In the current version, the fetcher engine is responsible of processing the assigned chunk locators.
// From the architectural perspective, AssignedChunkProcessor aligns as following on the verification pipeline:
// ----------------                                  ------------------                        ---------------------------
// | Chunk Queue  | ---> assigned chunk locators --> | Chunk Consumer | ---> chunk workers --> | Assigned Chunk Processor|
// ----------------                           		 ------------------                        ---------------------------
type AssignedChunkProcessor interface {
	module.ReadyDoneAware
	// ProcessAssignedChunk receives an assigned chunk locator and processes its corresponding chunk.
	// A chunk processor is expected to shape a verifiable chunk out of the assigned chunk, and pass it to
	// the verifier Engine.
	// Note: it should be implemented in a non-blocking way.
	ProcessAssignedChunk(locator *chunks.Locator)

	// WithChunkConsumerNotifier sets the notifier of this chunk processor.
	// The notifier is called by the internal logic of the processor to let the consumer know that
	// the processor is done by processing a chunk so that the next chunk may be passed to the processor
	// by the consumer through invoking ProcessAssignedChunk of this processor.
	WithChunkConsumerNotifier(notifier module.ProcessingNotifier)
}
