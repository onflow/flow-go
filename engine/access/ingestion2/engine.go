package ingestion2

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

const (
	// time to wait for the all the missing collections to be received at node startup
	collectionCatchupTimeout = 30 * time.Second

	// time to poll the storage to check if missing collections have been received
	collectionCatchupDBPollInterval = 10 * time.Millisecond

	// time to request missing collections from the network
	missingCollsRequestInterval = 1 * time.Minute

	// a threshold of number of blocks with missing collections beyond which collections should be re-requested
	// this is to prevent spamming the collection nodes with request
	missingCollsForBlockThreshold = 100

	// a threshold of block height beyond which collections should be re-requested (regardless of the number of blocks for which collection are missing)
	// this is to ensure that if a collection is missing for a long time (in terms of block height) it is eventually re-requested
	missingCollsForAgeThreshold = 100

	// time to update the FullBlockHeight index
	fullBlockRefreshInterval = 1 * time.Second

	defaultQueueCapacity = 10_000

	// processFinalizedBlocksWorkersCount defines the number of workers that
	// concurrently process finalized blocks in the job queue.
	processFinalizedBlocksWorkersCount = 1

	// searchAhead is a number of blocks that should be processed ahead by jobqueue
	searchAhead = 1
)

type Engine struct {
	*component.ComponentManager

	log zerolog.Logger

	executionReceiptConsumer *ExecutionReceiptConsumer

	finalizedBlockProcessor *FinalizedBlockProcessor

	errorMessageRequester ErrorMessageRequester

	collectionSyncer         *CollectionSyncer
	collectionExecutedMetric module.CollectionExecutedMetric
}

var _ network.MessageProcessor = (*Engine)(nil)

func New(
	log zerolog.Logger,
	net network.EngineRegistry,
	finalizedBlockProcessor *FinalizedBlockProcessor,
	executionReceiptConsumer *ExecutionReceiptConsumer,
	errorMessageRequester ErrorMessageRequester,
	collectionSyncer *CollectionSyncer,
	collectionExecutedMetric module.CollectionExecutedMetric,
) (*Engine, error) {
	e := &Engine{
		log:                      log.With().Str("engine", "ingestion2").Logger(),
		collectionExecutedMetric: collectionExecutedMetric,
		executionReceiptConsumer: executionReceiptConsumer,
		finalizedBlockProcessor:  finalizedBlockProcessor,
		errorMessageRequester:    errorMessageRequester,
		collectionSyncer:         collectionSyncer,
	}

	// Set up component manager
	builder := component.NewComponentManagerBuilder().
		AddWorker(e.executionReceiptConsumer.StartConsuming).
		AddWorker(e.finalizedBlockProcessor.StartProcessing).
		AddWorker(e.collectionSyncer.StartSyncing).
		AddWorker(e.errorMessageRequester.StartRequesting)
	e.ComponentManager = builder.Build()

	// engine gets execution receipts from channels.ReceiveReceipts channel
	_, err := net.Register(channels.ReceiveReceipts, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine in network to receive execution receipts: %w", err)
	}

	return e, nil
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(chanName channels.Channel, originID flow.Identifier, event interface{}) error {
	select {
	case <-e.ComponentManager.ShutdownSignal():
		return component.ErrComponentShutdown
	default:
	}

	switch event.(type) {
	case *flow.ExecutionReceipt:
		err := e.executionReceiptConsumer.Notify(originID, event)
		return err
	default:
		return fmt.Errorf("got invalid event type (%T) from %s channel", event, chanName)
	}
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated.
// Receives block finalized events from the finalization distributor and forwards them to the consumer.
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	e.finalizedBlockProcessor.Notify(block)
}
