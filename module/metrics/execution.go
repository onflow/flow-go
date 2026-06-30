package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/counters"
)

type ExecutionCollector struct {
	*LedgerCollector
	tracer                                  module.Tracer
	totalExecutedBlocksCounter              prometheus.Counter
	totalExecutedCollectionsCounter         prometheus.Counter
	totalExecutedTransactionsCounter        prometheus.Counter
	totalExecutedScriptsCounter             prometheus.Counter
	totalFailedTransactionsCounter          prometheus.Counter
	lastExecutedBlockHeightGauge            prometheus.Gauge
	lastFinalizedExecutedBlockHeightGauge   prometheus.Gauge
	lastChunkDataPackPrunedHeightGauge      prometheus.Gauge
	targetChunkDataPackPrunedHeightGauge    prometheus.Gauge
	stateStorageDiskTotal                   prometheus.Gauge
	storageStateCommitment                  prometheus.Gauge
	blockComputationUsed                    prometheus.Histogram
	blockComputationVector                  *prometheus.GaugeVec
	blockCachedPrograms                     prometheus.Gauge
	blockMemoryUsed                         prometheus.Histogram
	blockEventCounts                        prometheus.Histogram
	blockEventSize                          prometheus.Histogram
	blockExecutionTime                      prometheus.Histogram
	blockTransactionCounts                  prometheus.Histogram
	blockCollectionCounts                   prometheus.Histogram
	collectionComputationUsed               prometheus.Histogram
	collectionMemoryUsed                    prometheus.Histogram
	collectionEventSize                     prometheus.Histogram
	collectionEventCounts                   prometheus.Histogram
	collectionNumberOfRegistersTouched      prometheus.Histogram
	collectionTotalBytesWrittenToRegisters  prometheus.Histogram
	collectionExecutionTime                 prometheus.Histogram
	collectionTransactionCounts             prometheus.Histogram
	collectionRequestSent                   prometheus.Counter
	collectionRequestRetried                prometheus.Counter
	transactionParseTime                    prometheus.Histogram
	transactionCheckTime                    prometheus.Histogram
	transactionInterpretTime                prometheus.Histogram
	transactionExecutionTime                prometheus.Histogram
	transactionConflictRetries              prometheus.Histogram
	transactionMemoryEstimate               prometheus.Histogram
	transactionComputationUsed              prometheus.Histogram
	transactionNormalizedTimePerComputation prometheus.Histogram
	transactionEmittedEvents                prometheus.Histogram
	transactionEventSize                    prometheus.Histogram
	scriptExecutionTime                     prometheus.Histogram
	scriptComputationUsed                   prometheus.Histogram
	scriptMemoryUsage                       prometheus.Histogram
	scriptMemoryEstimate                    prometheus.Histogram
	scriptMemoryDifference                  prometheus.Histogram
	numberOfAccounts                        prometheus.Gauge
	programsCacheMiss                       prometheus.Counter
	programsCacheHit                        prometheus.Counter
	chunkDataPackRequestProcessedTotal      prometheus.Counter
	chunkDataPackProofSize                  prometheus.Histogram
	chunkDataPackCollectionSize             prometheus.Histogram
	stateSyncActive                         prometheus.Gauge
	blockDataUploadsInProgress              prometheus.Gauge
	blockDataUploadsDuration                prometheus.Histogram
	maxCollectionHeightData                 counters.StrictMonotonicCounter
	maxCollectionHeight                     prometheus.Gauge
	computationResultUploadedCount          prometheus.Counter
	computationResultUploadRetriedCount     prometheus.Counter
	numberOfDeployedCOAs                    prometheus.Gauge
	evmBlockTotalSupply                     prometheus.Gauge
	totalExecutedEVMTransactionsCounter     prometheus.Counter
	totalFailedEVMTransactionsCounter       prometheus.Counter
	totalExecutedEVMDirectCallsCounter      prometheus.Counter
	totalFailedEVMDirectCallsCounter        prometheus.Counter
	evmTransactionGasUsed                   prometheus.Histogram
	evmBlockTxCount                         prometheus.Histogram
	evmBlockGasUsed                         prometheus.Histogram
	callbacksExecutedCount                  prometheus.Histogram
	callbacksProcessComputationUsed         prometheus.Histogram
	callbacksExecuteComputationLimits       prometheus.Histogram
}

func NewExecutionCollector(tracer module.Tracer) *ExecutionCollector {
	// Create LedgerCollector with execution namespace and state_storage subsystem for checkpoint
	ledgerCollector := NewLedgerCollector(namespaceExecution, subsystemStateStorage)

	blockExecutionTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_execution_time_milliseconds",
		Help:      "the total time spent on block execution in milliseconds",
		Buckets:   []float64{50, 100, 200, 300, 400, 1000, 2000, 6000},
	})

	blockComputationUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_computation_used",
		Help:      "the total amount of computation used by a block",
		Buckets:   []float64{1000, 10000, 100000, 500000, 1000000, 5000000, 10000000},
	})

	blockMemoryUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_memory_used",
		Help:      "the total amount of memory (cadence estimate) used by a block",
		Buckets:   []float64{100_000_000, 1_000_000_000, 5_000_000_000, 10_000_000_000, 50_000_000_000, 100_000_000_000, 500_000_000_000, 1_000_000_000_000, 5_000_000_000_000, 10_000_000_000_000},
	})

	blockEventCounts := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_event_counts",
		Help:      "the total number of events emitted during a block execution",
		Buckets:   []float64{10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000},
	})

	blockEventSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_event_size",
		Help:      "the total number of bytes used by events emitted during a block execution",
		Buckets:   []float64{1_000, 10_000, 100_000, 500_000, 1_000_000, 5_000_000, 10_000_000, 50_000_000, 100_000_000, 500_000_000},
	})

	blockComputationVector := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_execution_effort_vector",
		Help:      "execution effort vector of the last executed block by computation kind",
	}, []string{LabelComputationKind})

	blockCachedPrograms := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_execution_cached_programs",
		Help:      "Number of cached programs at the end of block execution",
	})

	blockTransactionCounts := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_transaction_counts",
		Help:      "the total number of transactions per block",
		Buckets:   prometheus.ExponentialBuckets(4, 2, 10),
	})

	blockCollectionCounts := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "block_collection_counts",
		Help:      "the total number of collections per block",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 8),
	})

	collectionExecutionTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_execution_time_milliseconds",
		Help:      "the total time spent on collection execution in milliseconds",
		Buckets:   []float64{100, 200, 500, 1000, 1500, 2000},
	})

	collectionComputationUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_computation_used",
		Help:      "the total amount of computation used by a collection",
		Buckets:   []float64{1000, 10000, 50000, 100000, 500000, 1000000},
	})

	collectionMemoryUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_memory_used",
		Help:      "the total amount of memory used (cadence estimate) by a collection",
		Buckets:   []float64{10_000_000, 100_000_000, 1_000_000_000, 5_000_000_000, 10_000_000_000, 50_000_000_000, 100_000_000_000, 500_000_000_000, 1_000_000_000_000, 5_000_000_000_000},
	})

	collectionEventSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_event_size",
		Help:      "the total byte size used by all events generated during a collection execution",
		Buckets:   []float64{100, 1000, 10000, 100000, 10000000, 100000000, 1000000000},
	})

	collectionEventCounts := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_event_counts",
		Help:      "the total number of events emitted per collection",
		Buckets:   prometheus.ExponentialBuckets(4, 2, 8),
	})

	collectionNumberOfRegistersTouched := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_number_of_registers_touched",
		Help:      "the total number of registers touched during collection execution",
		Buckets:   prometheus.ExponentialBuckets(10, 2, 12),
	})

	collectionTotalBytesWrittenToRegisters := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_total_number_of_bytes_written_to_registers",
		Help:      "the total number of bytes written to registers during collection execution",
		Buckets:   prometheus.ExponentialBuckets(1000, 2, 16),
	})

	collectionTransactionCounts := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "collection_transaction_counts",
		Help:      "the total number of transactions per collection",
		Buckets:   prometheus.ExponentialBuckets(4, 2, 8),
	})

	collectionRequestsSent := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "collection_requests_sent",
		Help:      "the number of collection requests sent",
	})

	collectionRequestsRetries := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "collection_requests_retries",
		Help:      "the number of collection requests retried",
	})

	transactionParseTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_parse_time_nanoseconds",
		Help:      "the parse time for a transaction in nanoseconds",
		Buckets:   prometheus.ExponentialBuckets(10, 10, 8),
	})

	transactionCheckTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_check_time_nanoseconds",
		Help:      "the checking time for a transaction in nanoseconds",
		Buckets:   prometheus.ExponentialBuckets(10, 10, 8),
	})

	transactionInterpretTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_interpret_time_nanoseconds",
		Help:      "the interpretation time for a transaction in nanoseconds",
		Buckets:   prometheus.ExponentialBuckets(10, 10, 8),
	})

	transactionExecutionTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_execution_time_milliseconds",
		Help:      "the total time spent on transaction execution in milliseconds",
		Buckets:   prometheus.ExponentialBuckets(2, 2, 10),
	})

	transactionConflictRetries := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_conflict_retries",
		Help:      "the number of conflict retries needed to successfully commit a transaction.  If retry count is high, consider reducing concurrency",
		Buckets:   []float64{0, 1, 2, 3, 4, 5, 10, 20, 30, 40, 50, 100},
	})

	transactionComputationUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_computation_used",
		Help:      "the total amount of computation used by a transaction",
		Buckets:   []float64{50, 100, 500, 1000, 5000, 10000},
	})

	transactionNormalizedTimePerComputation := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_ms_per_computation",
		Help:      "The normalized ratio of millisecond of execution time per computation used. Value below 1 means the transaction was executed faster than estimated (is using less resources then estimated)",
		Buckets:   []float64{0.015625, 0.03125, 0.0625, 0.125, 0.25, 0.5, 1, 2, 4, 8, 16, 32, 64},
	})

	transactionMemoryEstimate := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_memory_estimate",
		Help:      "the estimated memory used by a transaction",
		Buckets:   []float64{1_000_000, 10_000_000, 100_000_000, 1_000_000_000, 5_000_000_000, 10_000_000_000, 50_000_000_000, 100_000_000_000},
	})

	transactionEmittedEvents := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_emitted_events",
		Help:      "the total number of events emitted by a transaction",
		Buckets:   prometheus.ExponentialBuckets(2, 2, 10),
	})

	transactionEventSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "transaction_event_size",
		Help:      "the total number bytes used of events emitted during a transaction execution",
		Buckets:   prometheus.ExponentialBuckets(100, 2, 12),
	})

	scriptExecutionTime := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "script_execution_time_milliseconds",
		Help:      "the total time spent on script execution in milliseconds",
		Buckets:   []float64{2, 4, 8, 16, 32, 64, 100, 250, 500},
	})

	scriptComputationUsed := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "script_computation_used",
		Help:      "the total amount of computation used by an script",
		Buckets:   []float64{50, 100, 500, 1000, 5000, 10000},
	})

	scriptMemoryUsage := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "script_memory_usage",
		Help:      "the total amount of memory allocated by a script",
		Buckets:   []float64{100_000, 1_000_000, 10_000_000, 50_000_000, 100_000_000, 500_000_000, 1_000_000_000},
	})

	scriptMemoryEstimate := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "script_memory_estimate",
		Help:      "the estimated memory used by a script",
		Buckets:   []float64{1_000_000, 10_000_000, 100_000_000, 1_000_000_000, 5_000_000_000, 10_000_000_000, 50_000_000_000, 100_000_000_000},
	})

	scriptMemoryDifference := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemRuntime,
		Name:      "script_memory_difference",
		Help:      "the difference in actual memory usage and estimate for a script",
		Buckets:   []float64{-1, 0, 10_000_000, 100_000_000, 1_000_000_000},
	})

	chunkDataPackRequestProcessedTotal := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemProvider,
		Name:      "chunk_data_packs_requested_total",
		Help:      "the total number of chunk data pack requests processed by provider engine",
	})

	chunkDataPackProofSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "chunk_data_pack_proof_size",
		Help:      "the total number bytes used for storing proof part of chunk data pack",
		Buckets:   prometheus.ExponentialBuckets(1000, 2, 16),
	})

	chunkDataPackCollectionSize := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemIngestion,
		Name:      "chunk_data_pack_collection_size",
		Help:      "the total number transactions in the collection",
		Buckets:   prometheus.ExponentialBuckets(1, 2, 10),
	})

	blockDataUploadsInProgress := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemBlockDataUploader,
		Name:      "block_data_upload_in_progress",
		Help:      "number of concurrently running Block Data upload operations",
	})

	blockDataUploadsDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemBlockDataUploader,
		Name:      "block_data_upload_duration_ms",
		Help:      "the duration of update upload operation",
		Buckets:   []float64{1, 100, 500, 1000, 2000},
	})

	computationResultUploadedCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemProvider,
		Name:      "computation_result_uploaded_count",
		Help:      "the total count of computation result uploaded",
	})

	computationResultUploadRetriedCount := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceExecution,
		Subsystem: subsystemProvider,
		Name:      "computation_result_upload_retried_count",
		Help:      "the total count of computation result upload retried",
	})

	ec := &ExecutionCollector{
		LedgerCollector:                         ledgerCollector,
		tracer:                                  tracer,
		blockExecutionTime:                      blockExecutionTime,
		blockComputationUsed:                    blockComputationUsed,
		blockComputationVector:                  blockComputationVector,
		blockCachedPrograms:                     blockCachedPrograms,
		blockMemoryUsed:                         blockMemoryUsed,
		blockEventCounts:                        blockEventCounts,
		blockEventSize:                          blockEventSize,
		blockTransactionCounts:                  blockTransactionCounts,
		blockCollectionCounts:                   blockCollectionCounts,
		collectionExecutionTime:                 collectionExecutionTime,
		collectionComputationUsed:               collectionComputationUsed,
		collectionMemoryUsed:                    collectionMemoryUsed,
		collectionEventSize:                     collectionEventSize,
		collectionEventCounts:                   collectionEventCounts,
		collectionNumberOfRegistersTouched:      collectionNumberOfRegistersTouched,
		collectionTotalBytesWrittenToRegisters:  collectionTotalBytesWrittenToRegisters,
		collectionTransactionCounts:             collectionTransactionCounts,
		collectionRequestSent:                   collectionRequestsSent,
		collectionRequestRetried:                collectionRequestsRetries,
		transactionParseTime:                    transactionParseTime,
		transactionCheckTime:                    transactionCheckTime,
		transactionInterpretTime:                transactionInterpretTime,
		transactionExecutionTime:                transactionExecutionTime,
		transactionConflictRetries:              transactionConflictRetries,
		transactionComputationUsed:              transactionComputationUsed,
		transactionNormalizedTimePerComputation: transactionNormalizedTimePerComputation,
		transactionMemoryEstimate:               transactionMemoryEstimate,
		transactionEmittedEvents:                transactionEmittedEvents,
		transactionEventSize:                    transactionEventSize,
		scriptExecutionTime:                     scriptExecutionTime,
		scriptComputationUsed:                   scriptComputationUsed,
		scriptMemoryUsage:                       scriptMemoryUsage,
		scriptMemoryEstimate:                    scriptMemoryEstimate,
		scriptMemoryDifference:                  scriptMemoryDifference,
		chunkDataPackRequestProcessedTotal:      chunkDataPackRequestProcessedTotal,
		chunkDataPackProofSize:                  chunkDataPackProofSize,
		chunkDataPackCollectionSize:             chunkDataPackCollectionSize,
		blockDataUploadsInProgress:              blockDataUploadsInProgress,
		blockDataUploadsDuration:                blockDataUploadsDuration,
		computationResultUploadedCount:          computationResultUploadedCount,
		computationResultUploadRetriedCount:     computationResultUploadRetriedCount,
		totalExecutedBlocksCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_executed_blocks",
			Help:      "the total number of blocks that have been executed",
		}),

		totalExecutedCollectionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_executed_collections",
			Help:      "the total number of collections that have been executed",
		}),

		totalExecutedTransactionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_executed_transactions",
			Help:      "the total number of transactions that have been executed",
		}),

		totalFailedTransactionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_failed_transactions",
			Help:      "the total number of transactions that has failed when executed",
		}),

		totalExecutedScriptsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "total_executed_scripts",
			Help:      "the total number of scripts that have been executed",
		}),

		lastExecutedBlockHeightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "last_executed_block_height",
			Help:      "the last height that was executed",
		}),

		lastFinalizedExecutedBlockHeightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "last_finalized_executed_block_height",
			Help:      "the last height that was finalized and executed",
		}),

		lastChunkDataPackPrunedHeightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "last_chunk_data_pack_pruned_height",
			Help:      "the last height that was pruned for chunk data pack",
		}),

		targetChunkDataPackPrunedHeightGauge: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "target_chunk_data_pack_pruned_height",
			Help:      "the target height for pruning chunk data pack",
		}),

		stateStorageDiskTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemStateStorage,
			Name:      "data_size_bytes",
			Help:      "the execution state size on disk in bytes",
		}),

		// TODO: remove
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

		numberOfAccounts: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "number_of_accounts",
			Help:      "the number of existing accounts on the network",
		}),

		programsCacheMiss: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "programs_cache_miss",
			Help:      "the number of times a program was not found in the cache and had to be loaded",
		}),

		programsCacheHit: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "programs_cache_hit",
			Help:      "the number of times a program was found in the cache",
		}),

		maxCollectionHeightData: counters.NewMonotonicCounter(0),

		maxCollectionHeight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name:      "max_collection_height",
			Namespace: namespaceExecution,
			Subsystem: subsystemIngestion,
			Help:      "gauge to track the maximum block height of collections received",
		}),

		numberOfDeployedCOAs: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "number_of_deployed_coas",
			Help:      "the number of deployed coas",
		}),

		totalExecutedEVMTransactionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "total_executed_evm_transaction_count",
			Help:      "the total number of executed evm transactions (including direct calls)",
		}),

		totalFailedEVMTransactionsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "total_failed_evm_transaction_count",
			Help:      "the total number of executed evm transactions with failed status (including direct calls)",
		}),

		totalExecutedEVMDirectCallsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "total_executed_evm_direct_call_count",
			Help:      "the total number of executed evm direct calls",
		}),

		totalFailedEVMDirectCallsCounter: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "total_failed_evm_direct_call_count",
			Help:      "the total number of executed evm direct calls with failed status.",
		}),

		evmTransactionGasUsed: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "evm_transaction_gas_used",
			Help:      "the total amount of gas used by a transaction",
			Buckets:   prometheus.ExponentialBuckets(20_000, 2, 8),
		}),

		evmBlockTxCount: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "evm_block_transaction_counts",
			Help:      "the total number of transactions per evm block",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 8),
		}),

		evmBlockGasUsed: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "evm_block_gas_used",
			Help:      "the total amount of gas used by a block",
			Buckets:   prometheus.ExponentialBuckets(100_000, 2, 8),
		}),

		evmBlockTotalSupply: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemEVM,
			Name:      "evm_block_total_supply",
			Help:      "the total amount of flow deposited to EVM (in FLOW)",
		}),

		callbacksExecutedCount: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "callbacks_executed_count",
			Help:      "the number of callbacks executed",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 9),
		}),

		callbacksProcessComputationUsed: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "callbacks_process_computation_used",
			Help:      "the computation used by the process callback transaction",
			Buckets:   prometheus.ExponentialBuckets(10_000, 2, 12),
		}),

		callbacksExecuteComputationLimits: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespaceExecution,
			Subsystem: subsystemRuntime,
			Name:      "callbacks_execute_computation_limits",
			Help:      "the total computation limits for execute callback transactions",
			Buckets:   prometheus.ExponentialBuckets(10_000, 2, 12),
		}),
	}

	return ec
}

// StartBlockReceivedToExecuted starts a span to trace the duration of a block
// from being received for execution to execution being finished
func (ec *ExecutionCollector) StartBlockReceivedToExecuted(blockID flow.Identifier) {
}

// FinishBlockReceivedToExecuted finishes a span to trace the duration of a block
// from being received for execution to execution being finished
func (ec *ExecutionCollector) FinishBlockReceivedToExecuted(blockID flow.Identifier) {
}

// ExecutionBlockExecuted reports execution meta data after executing a block
func (ec *ExecutionCollector) ExecutionBlockExecuted(
	dur time.Duration,
	stats module.BlockExecutionResultStats,
) {
	ec.totalExecutedBlocksCounter.Inc()
	ec.blockExecutionTime.Observe(float64(dur.Milliseconds()))
	ec.blockComputationUsed.Observe(float64(stats.ComputationUsed))
	ec.blockMemoryUsed.Observe(float64(stats.MemoryUsed))
	ec.blockEventCounts.Observe(float64(stats.EventCounts))
	ec.blockEventSize.Observe(float64(stats.EventSize))
	ec.blockTransactionCounts.Observe(float64(stats.NumberOfTransactions))
	ec.blockCollectionCounts.Observe(float64(stats.NumberOfCollections))
}

// ExecutionCollectionExecuted reports stats for executing a collection
func (ec *ExecutionCollector) ExecutionCollectionExecuted(
	dur time.Duration,
	stats module.CollectionExecutionResultStats,
) {
	ec.totalExecutedCollectionsCounter.Inc()
	ec.collectionExecutionTime.Observe(float64(dur.Milliseconds()))
	ec.collectionComputationUsed.Observe(float64(stats.ComputationUsed))
	ec.collectionMemoryUsed.Observe(float64(stats.MemoryUsed))
	ec.collectionEventCounts.Observe(float64(stats.EventCounts))
	ec.collectionEventSize.Observe(float64(stats.EventSize))
	ec.collectionNumberOfRegistersTouched.Observe(float64(stats.NumberOfRegistersTouched))
	ec.collectionTotalBytesWrittenToRegisters.Observe(float64(stats.NumberOfBytesWrittenToRegisters))
	ec.collectionTransactionCounts.Observe(float64(stats.NumberOfTransactions))
}

func (ec *ExecutionCollector) ExecutionBlockExecutionEffortVectorComponent(compKind string, value uint64) {
	ec.blockComputationVector.With(prometheus.Labels{LabelComputationKind: compKind}).Set(float64(value))
}

func (ec *ExecutionCollector) ExecutionBlockCachedPrograms(programs int) {
	ec.blockCachedPrograms.Set(float64(programs))
}

// ExecutionTransactionExecuted reports stats for executing a transaction
func (ec *ExecutionCollector) ExecutionTransactionExecuted(
	dur time.Duration,
	stats module.TransactionExecutionResultStats,
	_ module.TransactionExecutionResultInfo,
) {
	ec.totalExecutedTransactionsCounter.Inc()
	ec.transactionExecutionTime.Observe(float64(dur.Milliseconds()))
	ec.transactionConflictRetries.Observe(float64(stats.NumberOfTxnConflictRetries))
	ec.transactionComputationUsed.Observe(float64(stats.ComputationUsed))
	if stats.ComputationUsed > 0 {
		ec.transactionNormalizedTimePerComputation.Observe(
			flow.NormalizedExecutionTimePerComputationUnit(dur, stats.ComputationUsed))
	}
	ec.transactionMemoryEstimate.Observe(float64(stats.MemoryUsed))
	ec.transactionEmittedEvents.Observe(float64(stats.EventCounts))
	ec.transactionEventSize.Observe(float64(stats.EventSize))
	if stats.Failed {
		ec.totalFailedTransactionsCounter.Inc()
	}
}

// ExecutionChunkDataPackGenerated reports stats on chunk data pack generation
func (ec *ExecutionCollector) ExecutionChunkDataPackGenerated(proofSize, numberOfTransactions int) {
	ec.chunkDataPackProofSize.Observe(float64(proofSize))
	ec.chunkDataPackCollectionSize.Observe(float64(numberOfTransactions))
}

// ExecutionScriptExecuted reports the time spent executing a single script
func (ec *ExecutionCollector) ExecutionScriptExecuted(dur time.Duration, compUsed, memoryUsed, memoryEstimated uint64) {
	ec.totalExecutedScriptsCounter.Inc()
	ec.scriptExecutionTime.Observe(float64(dur.Milliseconds()))
	ec.scriptComputationUsed.Observe(float64(compUsed))
	ec.scriptMemoryUsage.Observe(float64(memoryUsed))
	ec.scriptMemoryEstimate.Observe(float64(memoryEstimated))
	ec.scriptMemoryDifference.Observe(float64(memoryEstimated) - float64(memoryUsed))
}

// ExecutionScheduledTransactionsExecuted reports scheduled transaction execution metrics
func (ec *ExecutionCollector) ExecutionScheduledTransactionsExecuted(scheduledTransactionCount int, processComputationUsed, executeComputationLimits uint64) {
	ec.callbacksExecutedCount.Observe(float64(scheduledTransactionCount))
	ec.callbacksProcessComputationUsed.Observe(float64(processComputationUsed))
	ec.callbacksExecuteComputationLimits.Observe(float64(executeComputationLimits))
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

// ExecutionLastFinalizedExecutedBlockHeight reports last finalized executed block height
func (ec *ExecutionCollector) ExecutionLastFinalizedExecutedBlockHeight(height uint64) {
	ec.lastFinalizedExecutedBlockHeightGauge.Set(float64(height))
}

// ExecutionLastChunkDataPackPrunedHeight reports last chunk data pack pruned height
func (ec *ExecutionCollector) ExecutionLastChunkDataPackPrunedHeight(height uint64) {
	ec.lastChunkDataPackPrunedHeightGauge.Set(float64(height))
}

func (ec *ExecutionCollector) ExecutionTargetChunkDataPackPrunedHeight(height uint64) {
	ec.targetChunkDataPackPrunedHeightGauge.Set(float64(height))
}

func (ec *ExecutionCollector) ExecutionCollectionRequestSent() {
	ec.collectionRequestSent.Inc()
}

func (ec *ExecutionCollector) ExecutionBlockDataUploadStarted() {
	ec.blockDataUploadsInProgress.Inc()
}

func (ec *ExecutionCollector) ExecutionBlockDataUploadFinished(dur time.Duration) {
	ec.blockDataUploadsInProgress.Dec()
	ec.blockDataUploadsDuration.Observe(float64(dur.Milliseconds()))
}

// TransactionParsed reports the time spent parsing a single transaction
func (ec *ExecutionCollector) RuntimeTransactionParsed(dur time.Duration) {
	ec.transactionParseTime.Observe(float64(dur))
}

// TransactionChecked reports the time spent checking a single transaction
func (ec *ExecutionCollector) RuntimeTransactionChecked(dur time.Duration) {
	ec.transactionCheckTime.Observe(float64(dur))
}

// TransactionInterpreted reports the time spent interpreting a single transaction
func (ec *ExecutionCollector) RuntimeTransactionInterpreted(dur time.Duration) {
	ec.transactionInterpretTime.Observe(float64(dur))
}

// ChunkDataPackRequestProcessed is executed every time a chunk data pack request is picked up for processing at execution node.
// It increases the request processed counter by one.
func (ec *ExecutionCollector) ChunkDataPackRequestProcessed() {
	ec.chunkDataPackRequestProcessedTotal.Inc()
}

func (ec *ExecutionCollector) ExecutionSync(syncing bool) {
	if syncing {
		ec.stateSyncActive.Set(float64(1))
		return
	}
	ec.stateSyncActive.Set(float64(0))
}

func (ec *ExecutionCollector) RuntimeSetNumberOfAccounts(count uint64) {
	ec.numberOfAccounts.Set(float64(count))
}

func (ec *ExecutionCollector) RuntimeTransactionProgramsCacheMiss() {
	ec.programsCacheMiss.Inc()
}

func (ec *ExecutionCollector) RuntimeTransactionProgramsCacheHit() {
	ec.programsCacheHit.Inc()
}

func (ec *ExecutionCollector) SetNumberOfDeployedCOAs(count uint64) {
	ec.numberOfDeployedCOAs.Set(float64(count))
}

func (ec *ExecutionCollector) EVMTransactionExecuted(
	gasUsed uint64,
	isDirectCall bool,
	failed bool,
) {
	ec.totalExecutedEVMTransactionsCounter.Inc()
	if isDirectCall {
		ec.totalExecutedEVMDirectCallsCounter.Inc()
		if failed {
			ec.totalFailedEVMDirectCallsCounter.Inc()
		}
	}
	if failed {
		ec.totalFailedEVMTransactionsCounter.Inc()
	}
	ec.evmTransactionGasUsed.Observe(float64(gasUsed))
}

func (ec *ExecutionCollector) EVMBlockExecuted(
	txCount int,
	totalGasUsed uint64,
	totalSupplyInFlow float64,
) {
	ec.evmBlockTxCount.Observe(float64(txCount))
	ec.evmBlockGasUsed.Observe(float64(totalGasUsed))
	ec.evmBlockTotalSupply.Set(totalSupplyInFlow)
}

func (ec *ExecutionCollector) UpdateCollectionMaxHeight(height uint64) {
	updated := ec.maxCollectionHeightData.Set(height)
	if updated {
		ec.maxCollectionHeight.Set(float64(height))
	}
}

func (ec *ExecutionCollector) ExecutionComputationResultUploaded() {
	ec.computationResultUploadedCount.Inc()
}

func (ec *ExecutionCollector) ExecutionComputationResultUploadRetried() {
	ec.computationResultUploadRetriedCount.Inc()
}

type ExecutionCollectorWithTransactionCallback struct {
	*ExecutionCollector
	TransactionCallback func(
		dur time.Duration,
		stats module.TransactionExecutionResultStats,
		info module.TransactionExecutionResultInfo,
	)
}

func (ec *ExecutionCollector) WithTransactionCallback(
	callback func(
		time.Duration,
		module.TransactionExecutionResultStats,
		module.TransactionExecutionResultInfo,
	),
) *ExecutionCollectorWithTransactionCallback {
	return &ExecutionCollectorWithTransactionCallback{
		ExecutionCollector:  ec,
		TransactionCallback: callback,
	}
}

func (ec *ExecutionCollectorWithTransactionCallback) ExecutionTransactionExecuted(
	dur time.Duration,
	stats module.TransactionExecutionResultStats,
	info module.TransactionExecutionResultInfo,
) {
	ec.ExecutionCollector.ExecutionTransactionExecuted(dur, stats, info)
	ec.TransactionCallback(dur, stats, info)
}
