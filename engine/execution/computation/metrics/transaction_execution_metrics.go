package metrics

import (
	"time"

	"github.com/onflow/flow-go/engine"

	cadenceCommon "github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	psEvents "github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

type TransactionExecutionMetricsProvider interface {
	component.Component
	protocol.Consumer

	// GetTransactionExecutionMetricsAfter returns the transaction metrics for all blocks higher than the given height
	// It returns a map of block height to a list of transaction execution metrics
	// Blocks that are out of scope (only a limited number blocks are kept in memory) are not returned
	GetTransactionExecutionMetricsAfter(height uint64) (GetTransactionExecutionMetricsAfterResponse, error)

	// Collect the transaction metrics for the given block
	// Collect does not block, it returns immediately
	Collect(
		blockId flow.Identifier,
		blockHeight uint64,
		t TransactionExecutionMetrics,
	)
}

// GetTransactionExecutionMetricsAfterResponse is the response type for GetTransactionExecutionMetricsAfter
// It is a map of block height to a list of transaction execution metrics
type GetTransactionExecutionMetricsAfterResponse = map[uint64][]TransactionExecutionMetrics

type TransactionExecutionMetrics struct {
	TransactionID          flow.Identifier
	ExecutionTime          time.Duration
	ExecutionEffortWeights map[cadenceCommon.ComputationKind]uint
}

type metrics struct {
	TransactionExecutionMetrics
	blockHeight uint64
	blockId     flow.Identifier
}

// transactionExecutionMetricsProvider is responsible for providing the metrics for the rpc endpoint.
// It has a circular buffer of metrics for the last N finalized and executed blocks.
// The metrics are not guaranteed to be available for all blocks. If the node is just starting up or catching up
// to the latest finalized block, some blocks may not have metrics available.
// The metrics are intended to be used for monitoring and analytics purposes.
type transactionExecutionMetricsProvider struct {
	// collector is responsible for collecting the metrics
	// the collector collects the metrics from the execution during block execution
	// on a finalized and executed block, the metrics are moved to the provider,
	// all non-finalized metrics for that height are discarded
	*collector

	// provider is responsible for providing the metrics for the rpc endpoint
	// it has a circular buffer of metrics for the last N finalized and executed blocks.
	*provider

	component.Component
	// transactionExecutionMetricsProvider needs to consume BlockFinalized events.
	psEvents.Noop

	log zerolog.Logger

	executionState         state.FinalizedExecutionState
	headers                storage.Headers
	blockFinalizedNotifier engine.Notifier

	latestFinalizedAndExecuted *flow.Header
}

var _ TransactionExecutionMetricsProvider = (*transactionExecutionMetricsProvider)(nil)

func NewTransactionExecutionMetricsProvider(
	log zerolog.Logger,
	executionState state.FinalizedExecutionState,
	headers storage.Headers,
	latestFinalizedBlock *flow.Header,
	bufferSize uint,
) TransactionExecutionMetricsProvider {
	log = log.With().Str("component", "transaction_execution_metrics_provider").Logger()

	collector := newCollector(log, latestFinalizedBlock.Height)
	provider := newProvider(log, bufferSize, latestFinalizedBlock.Height)

	p := &transactionExecutionMetricsProvider{
		collector:                  collector,
		provider:                   provider,
		log:                        log,
		executionState:             executionState,
		headers:                    headers,
		blockFinalizedNotifier:     engine.NewNotifier(),
		latestFinalizedAndExecuted: latestFinalizedBlock,
	}

	cm := component.NewComponentManagerBuilder()
	cm.AddWorker(collector.metricsCollectorWorker)
	cm.AddWorker(p.blockFinalizedWorker)

	p.Component = cm.Build()

	return p
}

func (p *transactionExecutionMetricsProvider) BlockFinalized(*flow.Header) {
	p.blockFinalizedNotifier.Notify()
}

// move data from the collector to the provider
func (p *transactionExecutionMetricsProvider) onBlockExecutedAndFinalized(block flow.Identifier, height uint64) {
	data := p.collector.Pop(height, block)
	p.provider.Push(height, data)
}

func (p *transactionExecutionMetricsProvider) blockFinalizedWorker(
	ctx irrecoverable.SignalerContext,
	ready component.ReadyFunc,
) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.blockFinalizedNotifier.Channel():
			p.onExecutedAndFinalized()
		}
	}
}

func (p *transactionExecutionMetricsProvider) onExecutedAndFinalized() {
	latestFinalizedAndExecutedHeight, err := p.executionState.GetHighestFinalizedExecuted()

	if err != nil {
		p.log.Warn().Err(err).Msg("could not get highest finalized executed")
		return
	}

	// the latest finalized and executed block could be more than one block further than the last one handled
	// step through all blocks between the last one handled and the latest finalized and executed
	for height := p.latestFinalizedAndExecuted.Height + 1; height <= latestFinalizedAndExecutedHeight; height++ {
		header, err := p.headers.ByHeight(height)
		if err != nil {
			p.log.Warn().
				Err(err).
				Uint64("height", height).
				Msg("could not get header by height")
			return
		}

		p.onBlockExecutedAndFinalized(header.ID(), height)

		if header.Height == latestFinalizedAndExecutedHeight {
			p.latestFinalizedAndExecuted = header
		}
	}
}
