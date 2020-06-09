package metrics

import (
	"time"

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
	forestApproxMemorySize           prometheus.Gauge
	forestNumberOfTrees              prometheus.Gauge
	updated                          prometheus.Counter
	proofSize                        prometheus.Gauge
	updatedValuesNumber              prometheus.Counter
	updatedValuesSize                prometheus.Gauge
	updatedDuration                  prometheus.Histogram
	updatedDurationPerValue          prometheus.Histogram
	readValuesNumber                 prometheus.Counter
	readValuesSize                   prometheus.Gauge
	readDuration                     prometheus.Histogram
	readDurationPerValue             prometheus.Histogram
	collectionRequestSent            prometheus.Counter
	collectionRequestRetried         prometheus.Counter
}

func NewExecutionCollector(tracer *trace.OpenTracer, registerer prometheus.Registerer) *ExecutionCollector {

	forestApproxMemorySize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "forest_approx_memory_size",
		Help:      "approximate size of in-memory forest in bytes",
	})

	forestNumberOfTrees := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "forest_number_of_trees",
		Help:      "number of trees in memory",
	})
	updatedCount := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "updates_counted",
		Help:      "number of updates",
	})

	proofSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "average_proof_size",
		Help:      "average size of a single generated proof in bytes",
	})

	updatedValuesNumber := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "update_values_number",
		Help:      "total number of values updated",
	})

	updatedValuesSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "update_values_size",
		Help:      "total size of values for single update in bytes",
	})
	updatedDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "update_duration",
		Help:      "duration of update operation",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	})
	updatedDurationPerValue := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "update_duration_per_Value",
		Help:      "duration of update operation per value",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	})
	readValuesNumber := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "read_values_number",
		Help:      "total number of values read",
	})
	readValuesSize := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "read_values_size",
		Help:      "total size of values for single read in bytes",
	})
	readDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "read_duration",
		Help:      "duration of read operation",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	})
	readDurationPerValue := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "read_duration_per_value",
		Help:      "duration of read operation per value",
		Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
	})

	collectionRequestsSent := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "collection_requests_sent",
		Help:      "number of collection requests sent",
	})

	collectionRequestsRetries := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "collection_requests_retries",
		Help:      "number of collection requests retried",
	})

	registerer.MustRegister(forestApproxMemorySize)
	registerer.MustRegister(forestNumberOfTrees)
	registerer.MustRegister(updatedCount)
	registerer.MustRegister(proofSize)
	registerer.MustRegister(updatedValuesNumber)
	registerer.MustRegister(updatedValuesSize)
	registerer.MustRegister(updatedDuration)
	registerer.MustRegister(updatedDurationPerValue)
	registerer.MustRegister(readValuesNumber)
	registerer.MustRegister(readValuesSize)
	registerer.MustRegister(readDuration)
	registerer.MustRegister(readDurationPerValue)
	registerer.MustRegister(collectionRequestsSent)
	registerer.MustRegister(collectionRequestsRetries)

	ec := &ExecutionCollector{
		tracer: tracer,

		forestApproxMemorySize:   forestApproxMemorySize,
		forestNumberOfTrees:      forestNumberOfTrees,
		updated:                  updatedCount,
		proofSize:                proofSize,
		updatedValuesNumber:      updatedValuesNumber,
		updatedValuesSize:        updatedValuesSize,
		updatedDuration:          updatedDuration,
		updatedDurationPerValue:  updatedDurationPerValue,
		readValuesNumber:         readValuesNumber,
		readValuesSize:           readValuesSize,
		readDuration:             readDuration,
		readDurationPerValue:     readDurationPerValue,
		collectionRequestSent:    collectionRequestsSent,
		collectionRequestRetried: collectionRequestsRetries,

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

// ForestApproxMemorySize records approximate memory usage of forest (all in-memory trees)
func (ec *ExecutionCollector) ForestApproxMemorySize(bytes uint64) {
	ec.forestApproxMemorySize.Set(float64(bytes))
}

// ForestNumberOfTrees current number of trees in a forest (in memory)
func (ec *ExecutionCollector) ForestNumberOfTrees(number uint64) {
	ec.forestNumberOfTrees.Set(float64(number))
}

// UpdateCount increase a counter of performed updates
func (ec *ExecutionCollector) UpdateCount() {
	ec.updated.Inc()
}

// ProofSize records a proof size
func (ec *ExecutionCollector) ProofSize(bytes uint32) {
	ec.proofSize.Set(float64(bytes))
}

// UpdateValuesNumber accumulates number of updated values
func (ec *ExecutionCollector) UpdateValuesNumber(number uint64) {
	ec.updatedValuesNumber.Add(float64(number))
}

// UpdateValuesSize total size (in bytes) of updates values
func (ec *ExecutionCollector) UpdateValuesSize(bytes uint64) {
	ec.updatedValuesSize.Set(float64(bytes))
}

// UpdateDuration records absolute time for the update of a trie
func (ec *ExecutionCollector) UpdateDuration(duration time.Duration) {
	ec.updatedDuration.Observe(duration.Seconds())
}

// UpdateDurationPerItem records update time for single value (total duration / number of updated values)
func (ec *ExecutionCollector) UpdateDurationPerItem(duration time.Duration) {
	ec.updatedDurationPerValue.Observe(duration.Seconds())
}

// ReadValuesNumber accumulates number of read values
func (ec *ExecutionCollector) ReadValuesNumber(number uint64) {
	ec.readValuesNumber.Add(float64(number))
}

// ReadValuesSize total size (in bytes) of read values
func (ec *ExecutionCollector) ReadValuesSize(bytes uint64) {
	ec.readValuesSize.Set(float64(bytes))
}

// ReadDuration records absolute time for the read from a trie
func (ec *ExecutionCollector) ReadDuration(duration time.Duration) {
	ec.readDuration.Observe(duration.Seconds())
}

// ReadDurationPerItem records read time for single value (total duration / number of read values)
func (ec *ExecutionCollector) ReadDurationPerItem(duration time.Duration) {
	ec.readDurationPerValue.Observe(duration.Seconds())
}

func (ec *ExecutionCollector) ExecutionCollectionRequestSent() {
	ec.collectionRequestSent.Inc()
}

func (ec *ExecutionCollector) ExecutionCollectionRequestRetried() {
	ec.collectionRequestRetried.Inc()
}
