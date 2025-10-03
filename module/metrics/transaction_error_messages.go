package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

var _ module.TransactionErrorMessagesMetrics = (*TransactionErrorMessagesCollector)(nil)

type TransactionErrorMessagesCollector struct {
	fetchDuration        prometheus.Histogram
	downloadsInProgress  prometheus.Gauge
	highestIndexedHeight prometheus.Gauge
	downloadRetries      prometheus.Counter
	failedDownloads      prometheus.Counter
}

func NewTransactionErrorMessagesCollector() *TransactionErrorMessagesCollector {

	fetchDuration := promauto.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemTxErrorFetcher,
		Name:      "tx_errors_download_duration_ms",
		Help:      "the duration of transaction error messages download operation",
		Buckets:   []float64{1, 100, 500, 1000, 2000, 5000, 20000},
	})

	highestIndexedHeight := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemTxErrorFetcher,
		Name:      "tx_errors_indexed_height",
		Help:      "highest consecutive finalized block height with all transaction errors indexed",
	})

	downloadsInProgress := promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemTxErrorFetcher,
		Name:      "tx_errors_in_progress_downloads",
		Help:      "number of concurrently running transaction error messages download operations",
	})

	downloadRetries := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemTxErrorFetcher,
		Name:      "tx_errors_download_retries_total",
		Help:      "number of transaction error messages download retries",
	})

	failedDownloads := promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespaceAccess,
		Subsystem: subsystemTxErrorFetcher,
		Name:      "tx_errors_failed_downloads_total",
		Help:      "number of failed transaction error messages downloads",
	})

	return &TransactionErrorMessagesCollector{
		fetchDuration:        fetchDuration,
		highestIndexedHeight: highestIndexedHeight,
		downloadsInProgress:  downloadsInProgress,
		downloadRetries:      downloadRetries,
		failedDownloads:      failedDownloads,
	}
}

func (c *TransactionErrorMessagesCollector) TxErrorsInitialHeight(height uint64) {
	c.highestIndexedHeight.Set(float64(height))
}

// TxErrorsFetchStarted records that a transaction error messages download has started.
func (c *TransactionErrorMessagesCollector) TxErrorsFetchStarted() {
	c.downloadsInProgress.Inc()
}

// TxErrorsFetchFinished records that a transaction error messages download has finished.
// Pass the highest consecutive height to ensure the metrics reflect the height up to which the
// requester has completed downloads. This allows us to easily see when downloading gets stuck.
func (c *TransactionErrorMessagesCollector) TxErrorsFetchFinished(duration time.Duration, success bool, height uint64) {
	c.downloadsInProgress.Dec()
	c.fetchDuration.Observe(float64(duration.Milliseconds()))
	if success {
		c.highestIndexedHeight.Set(float64(height))
	} else {
		c.failedDownloads.Inc()
	}
}

// TxErrorsFetchRetried records that a transaction error messages download has been retried.
func (c *TransactionErrorMessagesCollector) TxErrorsFetchRetried() {
	c.downloadRetries.Inc()
}
