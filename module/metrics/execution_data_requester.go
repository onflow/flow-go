package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type ExecutionDataRequesterCollector struct {
	fetchDuration prometheus.Histogram

	downloadsInProgress      prometheus.Gauge
	outstandingNotifications prometheus.Gauge

	highestNotificationHeight prometheus.Gauge
	highestDownloadHeight     prometheus.Gauge

	downloadRetries prometheus.Counter
	failedDownloads prometheus.Counter
}

func NewExecutionDataRequesterCollector() module.ExecutionDataRequesterMetrics {

	fetchDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_download_duration_ms",
		Help:      "the duration of execution data download operation",
		Buckets:   []float64{1, 100, 500, 1000, 2000, 5000},
	})

	downloadsInProgress := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_in_progress_downloads",
		Help:      "number of concurrently running execution data download operations",
	})

	outstandingNotifications := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_outstanding_notifications",
		Help:      "number of execution data received notifications waiting to be processed",
	})

	highestDownloadHeight := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_highest_download_height",
		Help:      "highest block height for which execution data has been received",
	})

	highestNotificationHeight := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_highest_notification_height",
		Help:      "highest block height for which execution data notifications have been sent",
	})

	downloadRetries := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_requester_download_retries_total",
		Help:      "number of execution data download retries",
	})

	failedDownloads := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceStateSync,
		Subsystem: subsystemExecutionDataRequester,
		Name:      "execution_data_failed_downloads_total",
		Help:      "number of failed execution data downloads",
	})

	return &ExecutionDataRequesterCollector{
		fetchDuration:             fetchDuration,
		downloadsInProgress:       downloadsInProgress,
		outstandingNotifications:  outstandingNotifications,
		highestDownloadHeight:     highestDownloadHeight,
		highestNotificationHeight: highestNotificationHeight,
		downloadRetries:           downloadRetries,
		failedDownloads:           failedDownloads,
	}
}

func (ec *ExecutionDataRequesterCollector) ExecutionDataFetchStarted() {
	ec.downloadsInProgress.Inc()
}

func (ec *ExecutionDataRequesterCollector) ExecutionDataFetchFinished(duration time.Duration, success bool, height uint64) {
	ec.downloadsInProgress.Dec()
	ec.fetchDuration.Observe(float64(duration.Milliseconds()))
	if success {
		ec.highestDownloadHeight.Set(float64(height))
		ec.outstandingNotifications.Inc()
	} else {
		ec.failedDownloads.Inc()
	}
}

func (ec *ExecutionDataRequesterCollector) NotificationSent(height uint64) {
	ec.outstandingNotifications.Dec()
	ec.highestNotificationHeight.Set(float64(height))
}

func (ec *ExecutionDataRequesterCollector) FetchRetried() {
	ec.downloadRetries.Inc()
}
