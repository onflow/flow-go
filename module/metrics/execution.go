package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// Execution spans.
const (
	// executionBlockReceivedToExecuted is a duration metric
	// from a block being received by an execution node to being executed
	executionBlockReceivedToExecuted = "execution_block_received_to_executed"
)

type ExecutionCollector struct {
	tracer                           module.Tracer
	computationUsedPerBlock          prometheus.Histogram
	stateReadsPerBlock               prometheus.Histogram
	totalExecutedTransactionsCounter prometheus.Counter
	lastExecutedBlockHeightGauge     prometheus.Gauge
	stateStorageDiskTotal            prometheus.Gauge
	storageStateCommitment           prometheus.Gauge
	forestApproxMemorySize           prometheus.Gauge
	forestNumberOfTrees              prometheus.Gauge
	latestTrieRegCount               prometheus.Gauge
	latestTrieRegCountDiff           prometheus.Gauge
	latestTrieMaxDepth               prometheus.Gauge
	latestTrieMaxDepthDiff           prometheus.Gauge
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
	transactionParseTime             prometheus.Histogram
	transactionCheckTime             prometheus.Histogram
	transactionInterpretTime         prometheus.Histogram
	totalChunkDataPackRequests       prometheus.Counter
	stateSyncActive                  prometheus.Gauge
	executionStateDiskUsage          prometheus.Gauge
}

func NewExecutionCollector(tracer module.Tracer, registerer prometheus.Registerer) *ExecutionCollector {

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

	latestTrieRegCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "latest_trie_reg_count",
		Help:      "number of allocated registers (latest created trie)",
	})

	latestTrieRegCountDiff := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "latest_trie_reg_count_diff",
		Help:      "the difference between number of unique register allocated of the latest created trie and parent trie",
	})

	latestTrieMaxDepth := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "latest_trie_max_depth",
		Help:      "maximum depth of the latest created trie",
	})

	latestTrieMaxDepthDiff := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemMTrie,
		Name:      "latest_trie_max_depth_diff",
		Help:      "the difference between the max depth of the latest created trie and parent trie",
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

	transactionParseTime := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_parse_time_nanoseconds",
		Help:      "the parse time for a transaction in nanoseconds",
	})

	transactionCheckTime := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_check_time_nanoseconds",
		Help:      "the checking time for a transaction in nanoseconds",
	})

	transactionInterpretTime := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_interpret_time_nanoseconds",
		Help:      "the interpretation time for a transaction in nanoseconds",
	})

	totalChunkDataPackRequests := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemProvider,
		Name:      "chunk_data_packs_requested_total",
		Help:      "total number of chunk data pack requests provider engine received",
	})

	registerer.MustRegister(forestApproxMemorySize)
	registerer.MustRegister(forestNumberOfTrees)
	registerer.MustRegister(latestTrieRegCount)
	registerer.MustRegister(latestTrieRegCountDiff)
	registerer.MustRegister(latestTrieMaxDepth)
	registerer.MustRegister(latestTrieMaxDepthDiff)
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
	registerer.MustRegister(transactionParseTime)
	registerer.MustRegister(transactionCheckTime)
	registerer.MustRegister(transactionInterpretTime)
	registerer.MustRegister(totalChunkDataPackRequests)

	ec := &ExecutionCollector{
		tracer: tracer,

		forestApproxMemorySize:     forestApproxMemorySize,
		forestNumberOfTrees:        forestNumberOfTrees,
		latestTrieRegCount:         latestTrieRegCount,
		latestTrieRegCountDiff:     latestTrieRegCountDiff,
		latestTrieMaxDepth:         latestTrieMaxDepth,
		latestTrieMaxDepthDiff:     latestTrieMaxDepthDiff,
		updated:                    updatedCount,
		proofSize:                  proofSize,
		updatedValuesNumber:        updatedValuesNumber,
		updatedValuesSize:          updatedValuesSize,
		updatedDuration:            updatedDuration,
		updatedDurationPerValue:    updatedDurationPerValue,
		readValuesNumber:           readValuesNumber,
		readValuesSize:             readValuesSize,
		readDuration:               readDuration,
		readDurationPerValue:       readDurationPerValue,
		collectionRequestSent:      collectionRequestsSent,
		collectionRequestRetried:   collectionRequestsRetries,
		transactionParseTime:       transactionParseTime,
		transactionCheckTime:       transactionCheckTime,
		transactionInterpretTime:   transactionInterpretTime,
		totalChunkDataPackRequests: totalChunkDataPackRequests,

		computationUsedPerBlock: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Buckets:   []float64{1}, //TODO(andrew) Set once there are some figures around compuation usage and limits
			Name:      "used_computation",
			Help:      "total computation used per block",
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

		lastExecutedBlockHeightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "last_executed_block_height",
			Help:      "the last height that was executed",
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

		stateSyncActive: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemIngestion,
			Name:      "state_sync_active",
			Help:      "indicates if the state sync is active",
		}),

		executionStateDiskUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemMTrie,
			Name:      "execution_state_disk_usage",
			Help:      "disk usage of execution state",
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

// ExecutionComputationUsedPerBlock reports total computation used per block
func (ec *ExecutionCollector) ExecutionComputationUsedPerBlock(inp uint64) {
	ec.computationUsedPerBlock.Observe(float64(inp))
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

// ExecutionLastExecutedBlockHeight reports last executed block height
func (ec *ExecutionCollector) ExecutionLastExecutedBlockHeight(height uint64) {
	ec.lastExecutedBlockHeightGauge.Set(float64(height))
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

// LatestTrieRegCount records the number of unique register allocated (the lastest created trie)
func (ec *ExecutionCollector) LatestTrieRegCount(number uint64) {
	ec.latestTrieRegCount.Set(float64(number))
}

// LatestTrieRegCountDiff records the difference between the number of unique register allocated of the latest created trie and parent trie
func (ec *ExecutionCollector) LatestTrieRegCountDiff(number uint64) {
	ec.latestTrieRegCountDiff.Set(float64(number))
}

// LatestTrieMaxDepth records the maximum depth of the last created trie
func (ec *ExecutionCollector) LatestTrieMaxDepth(number uint64) {
	ec.latestTrieMaxDepth.Set(float64(number))
}

// LatestTrieMaxDepthDiff records the difference between the max depth of the latest created trie and parent trie
func (ec *ExecutionCollector) LatestTrieMaxDepthDiff(number uint64) {
	ec.latestTrieMaxDepthDiff.Set(float64(number))
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

// TransactionParsed reports the time spent parsing a single transaction
func (ec *ExecutionCollector) TransactionParsed(dur time.Duration) {
	ec.transactionParseTime.Observe(float64(dur))
}

// TransactionChecked reports the time spent checking a single transaction
func (ec *ExecutionCollector) TransactionChecked(dur time.Duration) {
	ec.transactionCheckTime.Observe(float64(dur))
}

// TransactionInterpreted reports the time spent interpreting a single transaction
func (ec *ExecutionCollector) TransactionInterpreted(dur time.Duration) {
	ec.transactionInterpretTime.Observe(float64(dur))
}

// ChunkDataPackRequested is executed every time a chunk data pack request is arrived at execution node.
// It increases the request counter by one.
func (ec *ExecutionCollector) ChunkDataPackRequested() {
	ec.totalChunkDataPackRequests.Inc()
}

func (ec *ExecutionCollector) ExecutionSync(syncing bool) {
	if syncing {
		ec.stateSyncActive.Set(float64(1))
		return
	}
	ec.stateSyncActive.Set(float64(0))
}

func (ec *ExecutionCollector) DiskSize(bytes uint64) {
	ec.executionStateDiskUsage.Set(float64(bytes))
}
