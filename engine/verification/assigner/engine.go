package assigner

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// The Assigner engine reads the receipts from each finalized block.
// For each receipt, it reads its result and find the chunks the assigned
// to me to verify, and then save it to the chunks job queue for the
// fetcher engine to process.
type Engine struct {
	unit    *engine.Unit
	log     zerolog.Logger
	metrics module.VerificationMetrics
	me      module.Local
	state   protocol.State

	assigner         module.ChunkAssigner  // used to determine chunks this node needs to verify
	chunksQueue      storage.ChunksQueue   // to store chunks to be verified
	newChunkListener module.NewJobListener // to notify about a new chunk
	finishProcessing finishProcessing      // to report a block has been processed
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	me module.Local,
	state protocol.State,
	assigner module.ChunkAssigner,
	chunksQueue storage.ChunksQueue,
	newChunkListener module.NewJobListener,
) *Engine {
	e := &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "assigner").Logger(),
		metrics:          metrics,
		me:               me,
		state:            state,
		assigner:         assigner,
		chunksQueue:      chunksQueue,
		newChunkListener: newChunkListener,
	}

	return e
}

func (e *Engine) withFinishProcessing(finishProcessing finishProcessing) {
	e.finishProcessing = finishProcessing
}

func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	return nil
}

// ProcessFinalizedBlock process each finalized block, and find the chunks that
// assigned to me, and store it to the chunks job queue.
func (e *Engine) ProcessFinalizedBlock(block *flow.Block) {
	// the block consumer will pull as many finalized blocks as
	// it can consume to process
}
