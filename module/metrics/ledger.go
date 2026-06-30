package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

const (
	namespaceLedger = "ledger"
	subsystemWAL    = "wal"
)

// LedgerCollector implements both module.LedgerMetrics and module.WALMetrics.
// It can be reused by both the standalone ledger service and execution nodes.
type LedgerCollector struct {
	namespace    string
	walSubsystem string
	// LedgerMetrics
	forestApproxMemorySize    prometheus.Gauge
	forestNumberOfTrees       prometheus.Gauge
	latestTrieRegCount        prometheus.Gauge
	latestTrieRegCountDiff    prometheus.Gauge
	latestTrieRegSize         prometheus.Gauge
	latestTrieRegSizeDiff     prometheus.Gauge
	latestTrieMaxDepthTouched prometheus.Gauge
	updateCount               prometheus.Counter
	proofSize                 prometheus.Gauge
	updateValuesNumber        prometheus.Counter
	updateValuesSize          prometheus.Gauge
	updateDuration            prometheus.Histogram
	updateDurationPerItem     prometheus.Histogram
	readValuesNumber          prometheus.Counter
	readValuesSize            prometheus.Gauge
	readDuration              prometheus.Histogram
	readDurationPerItem       prometheus.Histogram

	// WALMetrics
	checkpointSize prometheus.Gauge
}

// NewLedgerCollector creates a new LedgerCollector that implements both
// module.LedgerMetrics and module.WALMetrics interfaces.
// If namespace is empty, it defaults to "ledger".
// If walSubsystem is empty, it defaults to "wal".
func NewLedgerCollector(namespace, walSubsystem string) *LedgerCollector {
	if namespace == "" {
		namespace = namespaceLedger
	}
	if walSubsystem == "" {
		walSubsystem = subsystemWAL
	}
	return &LedgerCollector{
		namespace:    namespace,
		walSubsystem: walSubsystem,
		forestApproxMemorySize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "forest_approx_memory_size",
			Help:      "an approximate size of in-memory forest in bytes",
		}),
		forestNumberOfTrees: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "forest_number_of_trees",
			Help:      "the number of trees in memory",
		}),
		latestTrieRegCount: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "latest_trie_reg_count",
			Help:      "the number of allocated registers (latest created trie)",
		}),
		latestTrieRegCountDiff: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "latest_trie_reg_count_diff",
			Help:      "the difference between number of unique register allocated of the latest created trie and parent trie",
		}),
		latestTrieRegSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "latest_trie_reg_size",
			Help:      "the size of allocated registers (latest created trie)",
		}),
		latestTrieRegSizeDiff: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "latest_trie_reg_size_diff",
			Help:      "the difference between size of unique register allocated of the latest created trie and parent trie",
		}),
		latestTrieMaxDepthTouched: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "latest_trie_max_depth_touched",
			Help:      "the maximum depth touched of the latest created trie",
		}),
		updateCount: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "updates_counted",
			Help:      "the number of updates",
		}),
		proofSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "average_proof_size",
			Help:      "the average size of a single generated proof in bytes",
		}),
		updateValuesNumber: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "update_values_number",
			Help:      "the total number of values updated",
		}),
		updateValuesSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "update_values_size",
			Help:      "the total size of values for single update in bytes",
		}),
		updateDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "update_duration",
			Help:      "the duration of update operation",
			Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
		}),
		updateDurationPerItem: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "update_duration_per_value",
			Help:      "the duration of update operation per value",
			Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
		}),
		readValuesNumber: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "read_values_number",
			Help:      "the total number of values read",
		}),
		readValuesSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "read_values_size",
			Help:      "the total size of values for single read in bytes",
		}),
		readDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "read_duration",
			Help:      "the duration of read operation",
			Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
		}),
		readDurationPerItem: promauto.NewHistogram(prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystemMTrie,
			Name:      "read_duration_per_value",
			Help:      "the duration of read operation per value",
			Buckets:   []float64{0.05, 0.2, 0.5, 1, 2, 5},
		}),
		checkpointSize: promauto.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: walSubsystem,
			Name:      "checkpoint_size_bytes",
			Help:      "the size of a checkpoint in bytes",
		}),
	}
}

// Ensure LedgerCollector implements both interfaces
var _ module.LedgerMetrics = (*LedgerCollector)(nil)
var _ module.WALMetrics = (*LedgerCollector)(nil)

// LedgerMetrics implementation

func (lc *LedgerCollector) ForestApproxMemorySize(bytes uint64) {
	lc.forestApproxMemorySize.Set(float64(bytes))
}

func (lc *LedgerCollector) ForestNumberOfTrees(number uint64) {
	lc.forestNumberOfTrees.Set(float64(number))
}

func (lc *LedgerCollector) LatestTrieRegCount(number uint64) {
	lc.latestTrieRegCount.Set(float64(number))
}

func (lc *LedgerCollector) LatestTrieRegCountDiff(number int64) {
	lc.latestTrieRegCountDiff.Set(float64(number))
}

func (lc *LedgerCollector) LatestTrieRegSize(size uint64) {
	lc.latestTrieRegSize.Set(float64(size))
}

func (lc *LedgerCollector) LatestTrieRegSizeDiff(size int64) {
	lc.latestTrieRegSizeDiff.Set(float64(size))
}

func (lc *LedgerCollector) LatestTrieMaxDepthTouched(maxDepth uint16) {
	lc.latestTrieMaxDepthTouched.Set(float64(maxDepth))
}

func (lc *LedgerCollector) UpdateCount() {
	lc.updateCount.Inc()
}

func (lc *LedgerCollector) ProofSize(bytes uint32) {
	lc.proofSize.Set(float64(bytes))
}

func (lc *LedgerCollector) UpdateValuesNumber(number uint64) {
	lc.updateValuesNumber.Add(float64(number))
}

func (lc *LedgerCollector) UpdateValuesSize(bytes uint64) {
	lc.updateValuesSize.Set(float64(bytes))
}

func (lc *LedgerCollector) UpdateDuration(duration time.Duration) {
	lc.updateDuration.Observe(duration.Seconds())
}

func (lc *LedgerCollector) UpdateDurationPerItem(duration time.Duration) {
	lc.updateDurationPerItem.Observe(duration.Seconds())
}

func (lc *LedgerCollector) ReadValuesNumber(number uint64) {
	lc.readValuesNumber.Add(float64(number))
}

func (lc *LedgerCollector) ReadValuesSize(bytes uint64) {
	lc.readValuesSize.Set(float64(bytes))
}

func (lc *LedgerCollector) ReadDuration(duration time.Duration) {
	lc.readDuration.Observe(duration.Seconds())
}

func (lc *LedgerCollector) ReadDurationPerItem(duration time.Duration) {
	lc.readDurationPerItem.Observe(duration.Seconds())
}

// WALMetrics implementation

func (lc *LedgerCollector) ExecutionCheckpointSize(bytes uint64) {
	lc.checkpointSize.Set(float64(bytes))
}
