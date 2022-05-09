package metrics

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type ExecutionDataRequesterCollector struct {
	fulfilledHeight               prometheus.Gauge
	receiptsSkipped               prometheus.Counter
	requestDurations              prometheus.Summary
	latestSuccessfulRequestHeight prometheus.Gauge
	executionDataSizes            prometheus.Summary
	requestAttempts               prometheus.Histogram
	requestsFailed                *prometheus.CounterVec
	requestsCancelled             prometheus.Counter
	resultsDropped                prometheus.Counter
}

func NewExecutionDataRequesterCollector() *ExecutionDataRequesterCollector {
	return &ExecutionDataRequesterCollector{
		fulfilledHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "fulfilled_height",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the latest fulfilled height",
		}),
		receiptsSkipped: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "receipts_skipped",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the number of skipped receipts",
		}),
		requestDurations: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "request_durations_ms",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the durations of requests in milliseconds",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		latestSuccessfulRequestHeight: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "latest_successful_request_height",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the block height of the latest successful request",
		}),
		executionDataSizes: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "execution_data_sizes_bytes",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the sizes of Block Execution Data in bytes",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		requestAttempts: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "request_attempts",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Buckets:   []float64{1, 2, 3, 4, 5},
			Help:      "the number of attempts before a request succeeded",
		}),
		requestsFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "requests_failed",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the number of failed requests",
		}, []string{ExecutionDataRequestRetryable}),
		requestsCancelled: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "requests_cancelled",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the number of cancelled requests",
		}),
		resultsDropped: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "results_dropped",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataRequester,
			Help:      "the number of dropped responses",
		}),
	}
}

func (c *ExecutionDataRequesterCollector) FulfilledHeight(blockHeight uint64) {
	c.fulfilledHeight.Set(float64(blockHeight))
}

func (c *ExecutionDataRequesterCollector) ReceiptSkipped() {
	c.receiptsSkipped.Inc()
}

func (c *ExecutionDataRequesterCollector) RequestSucceeded(blockHeight uint64, duration time.Duration, totalSize uint64, numberOfAttempts int) {
	c.requestDurations.Observe(float64(duration.Milliseconds()))
	c.latestSuccessfulRequestHeight.Set(float64(blockHeight))
	c.executionDataSizes.Observe(float64(totalSize))
	c.requestAttempts.Observe(float64(numberOfAttempts))
}

func (c *ExecutionDataRequesterCollector) RequestFailed(duration time.Duration, retryable bool) {
	c.requestDurations.Observe(float64(duration.Milliseconds()))
	c.requestsFailed.WithLabelValues(strconv.FormatBool(retryable)).Inc()
}

func (c *ExecutionDataRequesterCollector) RequestCanceled() {
	c.requestsCancelled.Inc()
}

func (c *ExecutionDataRequesterCollector) ResultDropped() {
	c.resultsDropped.Inc()
}

type ExecutionDataProviderCollector struct {
	computeRootIDDurations prometheus.Summary
	numberOfChunks         prometheus.Histogram
	addBlobsDurations      prometheus.Summary
	executionDataSizes     prometheus.Summary
	addBlobsFailed         prometheus.Counter
}

func NewExecutionDataProviderCollector() *ExecutionDataProviderCollector {
	return &ExecutionDataProviderCollector{
		computeRootIDDurations: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "compute_root_id_durations_ms",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataProvider,
			Help:      "the durations of computing root IDs in milliseconds",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		numberOfChunks: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "number_of_chunks",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataProvider,
			Buckets:   []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25},
			Help:      "the number of chunks in a Block Execution Data",
		}),
		addBlobsDurations: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "add_blobs_durations_ms",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataProvider,
			Help:      "the durations of adding blobs in milliseconds",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		executionDataSizes: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "execution_data_sizes_bytes",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataProvider,
			Help:      "the sizes of Block Execution Data in bytes",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		addBlobsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "add_blobs_failed",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataProvider,
			Help:      "the number of failed attempts to add blobs",
		}),
	}
}

func (c *ExecutionDataProviderCollector) RootIDComputed(duration time.Duration, numberOfChunks int) {
	c.computeRootIDDurations.Observe(float64(duration.Milliseconds()))
	c.numberOfChunks.Observe(float64(numberOfChunks))
}

func (c *ExecutionDataProviderCollector) AddBlobsSucceeded(duration time.Duration, totalSize uint64) {
	c.addBlobsDurations.Observe(float64(duration.Milliseconds()))
	c.executionDataSizes.Observe(float64(totalSize))
}

func (c *ExecutionDataProviderCollector) AddBlobsFailed() {
	c.addBlobsFailed.Inc()
}

type ExecutionDataPrunerCollector struct {
	pruneDurations     prometheus.Summary
	latestHeightPruned prometheus.Gauge
}

func NewExecutionDataPrunerCollector() *ExecutionDataPrunerCollector {
	return &ExecutionDataPrunerCollector{
		pruneDurations: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "prune_durations_ms",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataPruner,
			Help:      "the durations of pruning in milliseconds",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.1:  0.01,
				0.25: 0.025,
				0.5:  0.05,
				0.75: 0.025,
				0.9:  0.01,
				0.99: 0.001,
			},
		}),
		latestHeightPruned: promauto.NewGauge(prometheus.GaugeOpts{
			Name:      "latest_height_pruned",
			Namespace: namespaceExecutionDataSync,
			Subsystem: subsystemExeDataPruner,
			Help:      "the latest height pruned",
		}),
	}
}

func (c *ExecutionDataPrunerCollector) Pruned(height uint64, duration time.Duration) {
	c.pruneDurations.Observe(float64(duration.Milliseconds()))
	c.latestHeightPruned.Set(float64(height))
}
