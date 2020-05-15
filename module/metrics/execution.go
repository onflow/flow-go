package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/trace"
)

// Execution spans.
const (
	// executionBlockReceivedToExecuted is a duration metric
	// from a block being received by an execution node to being executed
	executionBlockReceivedToExecuted = "execution_block_received_to_executed"
)

type ExecutionCollector struct {
	tracer                           *trace.OpenTracer
	gasUsedPerBlock                  prometheus.Histogram
	stateReadsPerBlock               prometheus.Histogram
	totalExecutedTransactionsCounter prometheus.Counter
	lastExecutedBlockViewGauge       prometheus.Gauge
	stateStorageDiskTotal            prometheus.Gauge
	storageStateCommitment           prometheus.Gauge
}

func NewExecutionCollector(tracer *trace.OpenTracer) *ExecutionCollector {

	ec := &ExecutionCollector{
		tracer: tracer,

		gasUsedPerBlock: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Buckets:   []float64{1}, //TODO(andrew) Set once there are some figures around gas usage and limits
			Name:      "used_gas",
			Help:      "the gas used per block",
		}),

		stateReadsPerBlock: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Buckets:   []float64{5, 10, 50, 100, 500},
			Name:      "block_state_reads",
			Help:      "count of state access/read operations performed per block",
		}),

		totalExecutedTransactionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_executed_transactions",
			Help:      "the total number of transactions that have been executed",
		}),

		lastExecutedBlockViewGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "last_executed_block_view",
			Help:      "the last view that was executed",
		}),

		stateStorageDiskTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemStateStorage,
			Name:      "data_size_bytes",
			Help:      "the execution state size on disk in bytes",
		}),

		storageStateCommitment: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemStateStorage,
			Name:      "commitment_size_bytes",
			Help:      "the storage size of a state commitment in bytes",
		}),
	}

	return ec
}

// StartBlockReceivedToExecuted starts a span to trace the duration of a block
// from being received for execution to execution being finished
func (ec *ExecutionCollector) StartBlockReceivedToExecuted(blockID flow.Identifier) {
	ec.tracer.StartSpan(blockID, executionBlockReceivedToExecuted).SetTag("block_id", blockID.String)
}

// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
// from being received for execution to execution being finished
func (ec *ExecutionCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier) {
	ec.tracer.FinishSpan(blockID, executionBlockReceivedToExecuted)
}

// ExecutionGasUsedPerBlock reports gas used per block
func (ec *ExecutionCollector) ExecutionGasUsedPerBlock(gas uint64) {
	ec.gasUsedPerBlock.Observe(float64(gas))
}

// ExecutionStateReadsPerBlock reports number of state access/read operations per block
func (ec *ExecutionCollector) ExecutionStateReadsPerBlock(reads uint64) {
	ec.stateReadsPerBlock.Observe(float64(reads))
}

// ExecutionStateStorageDiskTotal reports the total storage size of the execution state on disk in bytes
func (ec *ExecutionCollector) ExecutionStateStorageDiskTotal(bytes int64) {
	ec.stateStorageDiskTotal.Set(float64(bytes))
}

// ExecutionStorageStateCommitment reports the storage size of a state commitment
func (ec *ExecutionCollector) ExecutionStorageStateCommitment(bytes int64) {
	ec.storageStateCommitment.Set(float64(bytes))
}

// ExecutionLastExecutedBlockView reports last executed block view
func (ec *ExecutionCollector) ExecutionLastExecutedBlockView(view uint64) {
	ec.lastExecutedBlockViewGauge.Set(float64(view))
}

// ExecutionTotalExecutedTransactions reports total executed transactions
func (ec *ExecutionCollector) ExecutionTotalExecutedTransactions(numberOfTx int) {
	ec.totalExecutedTransactionsCounter.Add(float64(numberOfTx))
}
