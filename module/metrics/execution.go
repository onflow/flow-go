package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Execution Metrics
const (
	// executionBlockReceivedToExecuted is a duration metric
	// from a block being received by an execution node to being executed
	executionBlockReceivedToExecuted = "execution_block_received_to_executed"
)

var (
	executionGasUsedPerBlockHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Buckets:   []float64{1}, //TODO(andrew) Set once there are some figures around gas usage and limits
		Name:      "used_gas",
		Help:      "the gas used per block",
	})
	executionStateReadsPerBlockHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Buckets:   []float64{5, 10, 50, 100, 500},
		Name:      "block_state_reads",
		Help:      "count of state access/read operations performed per block",
	})
	executionStateStorageDiskTotalGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemStateStorage,
		Name:      "data_size_bytes",
		Help:      "the execution state size on disk in bytes",
	})
	executionStorageStateCommitmentGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemStateStorage,
		Name:      "commitment_size_bytes",
		Help:      "the storage size of a state commitment in bytes",
	})
	executionTransactionParseTimeHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemVirtualMachine,
		Name:      "transaction_parse_time_nanoseconds",
		Help:      "the parse time for a transaction in nanoseconds",
	})
	executionTransactionCheckTimeHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemVirtualMachine,
		Name:      "transaction_check_time_nanoseconds",
		Help:      "the checking time for a transaction in nanoseconds",
	})
	executionTransactionInterpretationTimeHist = promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemVirtualMachine,
		Name:      "transaction_interpret_time_nanoseconds",
		Help:      "the interpretation time for a transaction in nanoseconds",
	})
)

// StartBlockReceivedToExecuted starts a span to trace the duration of a block
// from being received for execution to execution being finished
func (c *Collector) StartBlockReceivedToExecuted(blockID flow.Identifier) {
	c.tracer.StartSpan(blockID, executionBlockReceivedToExecuted).
		SetTag("block_id", blockID.String)
}

// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
// from being received for execution to execution being finished
func (c *Collector) FinishBlockReceivedToExecuted(blockID flow.Identifier) {
	c.tracer.FinishSpan(blockID, executionBlockReceivedToExecuted)
}

// ExecutionGasUsedPerBlock reports gas used per block
func (c *Collector) ExecutionGasUsedPerBlock(gas uint64) {
	executionGasUsedPerBlockHist.Observe(float64(gas))
}

// ExecutionStateReadsPerBlock reports number of state access/read operations per block
func (c *Collector) ExecutionStateReadsPerBlock(reads uint64) {
	executionStateReadsPerBlockHist.Observe(float64(reads))
}

// ExecutionStateStorageDiskTotal reports the total storage size of the execution state on disk in bytes
func (c *Collector) ExecutionStateStorageDiskTotal(bytes int64) {
	executionStateStorageDiskTotalGauge.Set(float64(bytes))
}

// ExecutionStorageStateCommitment reports the storage size of a state commitment
func (c *Collector) ExecutionStorageStateCommitment(bytes int64) {
	executionStorageStateCommitmentGauge.Set(float64(bytes))
}

// ExecutionTransactionParsed reports the parse time of a transaction
func (c *Collector) ExecutionTransactionParsed(dur time.Duration) {
	executionTransactionParseTimeHist.Observe(float64(dur))
}

// ExecutionTransactionChecked reports the checking time of a transaction
func (c *Collector) ExecutionTransactionChecked(dur time.Duration) {
	executionTransactionCheckTimeHist.Observe(float64(dur))
}

// ExecutionTransactionInterpreted reports the interpretation time of a transaction
func (c *Collector) ExecutionTransactionInterpreted(dur time.Duration) {
	executionTransactionInterpretationTimeHist.Observe(float64(dur))
}
