package verification

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
)

// IngestEngine represents the ingest engine of the verification node. It is
// responsible for receiving and handling new execution receipts. It requests
// all dependent resources for each execution receipt and relays a complete
// execution result to the verifier engine when all dependencies are ready.
// Any implementation of IngestEngine should implement following interfaces:
// * Engine
// * FinalizationConsumer
// * ReadyDoneAware
type IngestEngine interface {
	// Engine related methods
	//
	// SubmitLocal submits an event originating on the local node.
	SubmitLocal(event interface{})

	// Submit submits the given event from the node with the given origin ID
	// for processing in a non-blocking manner. It returns instantly and logs
	// a potential processing error internally when done.
	Submit(originID flow.Identifier, event interface{})

	// ProcessLocal processes an event originating on the local node.
	ProcessLocal(event interface{}) error

	// Process processes the given event from the node with the given origin ID
	// in a blocking manner. It returns the potential processing error when
	// done.
	Process(originID flow.Identifier, event interface{}) error

	// FinalizationConsumer
	//
	// OnBlockIncorporated notifications are produced by the Finalization Logic
	// whenever a block is incorporated into the consensus state.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnBlockIncorporated(*model.Block)

	// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
	// a block has been finalized. They are emitted in the order the blocks are finalized.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnFinalizedBlock(block *model.Block)

	// OnDoubleProposeDetected notifications are produced by the Finalization Logic
	// whenever a double block proposal (equivocation) was detected.
	// Prerequisites:
	// Implementation must be concurrency safe; Non-blocking;
	// and must handle repetition of the same events (with some processing overhead).
	OnDoubleProposeDetected(*model.Block, *model.Block)

	// ReadyDoneAware
	//
	Ready() <-chan struct{}
	Done() <-chan struct{}
}
