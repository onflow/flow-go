package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
)

type TransactionCollector struct {
	transactionTimings             mempool.TransactionTimings
	log                            zerolog.Logger
	logTimeToFinalized             bool
	logTimeToExecuted              bool
	logTimeToFinalizedExecuted     bool
	timeToFinalized                prometheus.Summary
	timeToExecuted                 prometheus.Summary
	timeToFinalizedExecuted        prometheus.Summary
	transactionSubmission          *prometheus.CounterVec
	transactionSize                prometheus.Histogram
	scriptExecutedDuration         *prometheus.HistogramVec
	scriptExecutionErrorOnExecutor *prometheus.CounterVec
	scriptExecutionComparison      *prometheus.CounterVec
	scriptSize                     prometheus.Histogram
	transactionResultDuration      *prometheus.HistogramVec
}

// interface check
var _ module.BackendScriptsMetrics = (*TransactionCollector)(nil)
var _ module.TransactionMetrics = (*TransactionCollector)(nil)

func NewTransactionCollector(
	log zerolog.Logger,
	transactionTimings mempool.TransactionTimings,
	logTimeToFinalized bool,
	logTimeToExecuted bool,
	logTimeToFinalizedExecuted bool,
) *TransactionCollector {

	tc := &TransactionCollector{
		transactionTimings:         transactionTimings,
		log:                        log,
		logTimeToFinalized:         logTimeToFinalized,
		logTimeToExecuted:          logTimeToExecuted,
		logTimeToFinalizedExecuted: logTimeToFinalizedExecuted,
		timeToFinalized: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "time_to_finalized_seconds",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionTiming,
			Help:      "the duration of how long it took between the transaction was received until it was finalized",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.5:  0.05,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
		timeToExecuted: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "time_to_executed_seconds",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionTiming,
			Help:      "the duration of how long it took between the transaction was received until it was executed",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.5:  0.05,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
		timeToFinalizedExecuted: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "time_to_finalized_executed_seconds",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionTiming,
			Help: "the duration of how long it took between the transaction was received until it was both " +
				"finalized and executed",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.5:  0.05,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
		transactionSubmission: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_submission",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "counter for the success/failure of transaction submissions",
		}, []string{"result"}),
		scriptExecutedDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "script_executed_duration",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "histogram for the duration in ms of the round trip time for executing a script",
			Buckets:   []float64{1, 100, 500, 1000, 2000, 5000},
		}, []string{"script_size"}),
		scriptExecutionErrorOnExecutor: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "script_execution_error_executor",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "counter for the internal errors while executing a script",
		}, []string{"source"}),
		scriptExecutionComparison: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "script_execution_comparison",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "counter for the comparison outcomes of executing a script locally and on execution node",
		}, []string{"outcome"}),
		transactionResultDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:      "transaction_result_fetched_duration",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "histogram for the duration in ms of the round trip time for getting a transaction result",
			Buckets:   []float64{1, 100, 500, 1000, 2000, 5000},
		}, []string{"payload_size"}),
		scriptSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "script_size",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "histogram for the script size in kb of scripts used in ExecuteScript",
		}),
		transactionSize: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:      "transaction_size",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "histogram for the transaction size in kb of scripts used in GetTransactionResult",
		}),
	}

	return tc
}

// Script exec metrics

func (tc *TransactionCollector) ScriptExecuted(dur time.Duration, size int) {
	// record the execute script duration and script size
	tc.scriptSize.Observe(float64(size / 1024))
	tc.scriptExecutedDuration.With(prometheus.Labels{
		"script_size": tc.sizeLabel(size),
	}).Observe(float64(dur.Milliseconds()))
}

func (tc *TransactionCollector) ScriptExecutionErrorLocal() {
	// record the execution error count
	tc.scriptExecutionErrorOnExecutor.WithLabelValues("local").Inc()
}

func (tc *TransactionCollector) ScriptExecutionErrorOnExecutionNode() {
	// record the execution error count
	tc.scriptExecutionErrorOnExecutor.WithLabelValues("execution").Inc()
}

func (tc *TransactionCollector) ScriptExecutionResultMismatch() {
	// record the execution error count
	tc.scriptExecutionComparison.WithLabelValues("result_mismatch").Inc()
}

func (tc *TransactionCollector) ScriptExecutionResultMatch() {
	// record the execution error count
	tc.scriptExecutionComparison.WithLabelValues("result_match").Inc()
}
func (tc *TransactionCollector) ScriptExecutionErrorMismatch() {
	// record the execution error count
	tc.scriptExecutionComparison.WithLabelValues("error_mismatch").Inc()
}

func (tc *TransactionCollector) ScriptExecutionErrorMatch() {
	// record the execution error count
	tc.scriptExecutionComparison.WithLabelValues("error_match").Inc()
}

// ScriptExecutionNotIndexed records script execution matches where data for the block is not
// indexed locally yet
func (tc *TransactionCollector) ScriptExecutionNotIndexed() {
	tc.scriptExecutionComparison.WithLabelValues("not_indexed").Inc()
}

// TransactionResult metrics

func (tc *TransactionCollector) TransactionResultFetched(dur time.Duration, size int) {
	// record the transaction result duration and transaction script/payload size
	tc.transactionSize.Observe(float64(size / 1024))
	tc.transactionResultDuration.With(prometheus.Labels{
		"payload_size": tc.sizeLabel(size),
	}).Observe(float64(dur.Milliseconds()))
}

func (tc *TransactionCollector) sizeLabel(size int) string {
	// determine the script size label using the size in bytes
	sizeKb := size / 1024
	sizeLabel := "100+kb" //"1kb,10kb,100kb, 100+kb" -> [0,1] [2,10] [11,100] [100, +inf]

	if sizeKb <= 1 {
		sizeLabel = "1kb"
	} else if sizeKb <= 10 {
		sizeLabel = "10kb"
	} else if sizeKb <= 100 {
		sizeLabel = "100kb"
	}
	return sizeLabel
}

func (tc *TransactionCollector) TransactionReceived(txID flow.Identifier, when time.Time) {
	// we don't need to check whether the transaction timing already exists, it will not be overwritten by the mempool
	added := tc.transactionTimings.Add(&flow.TransactionTiming{TransactionID: txID, Received: when})
	if !added {
		tc.log.Warn().
			Str("transaction_id", txID.String()).
			Msg("failed to add TransactionReceived metric")
	}
}

func (tc *TransactionCollector) TransactionFinalized(txID flow.Identifier, when time.Time) {
	// Count as submitted as long as it's finalized
	tc.transactionSubmission.WithLabelValues("success").Inc()

	t, updated := tc.transactionTimings.Adjust(txID, func(t *flow.TransactionTiming) *flow.TransactionTiming {
		t.Finalized = when
		return t
	})

	// the AN may not have received the original transaction sent by the client in which case the finalized metric
	// is not updated
	if !updated {
		tc.log.Debug().
			Str("transaction_id", txID.String()).
			Msg("failed to update TransactionFinalized metric")
		return
	}

	tc.trackTTF(t, tc.logTimeToFinalized)
	tc.trackTTFE(t, tc.logTimeToFinalizedExecuted)

	// remove transaction timing from mempool if finalized and executed
	if !t.Finalized.IsZero() && !t.Executed.IsZero() {
		tc.transactionTimings.Remove(txID)
	}
}

func (tc *TransactionCollector) TransactionExecuted(txID flow.Identifier, when time.Time) {
	t, updated := tc.transactionTimings.Adjust(txID, func(t *flow.TransactionTiming) *flow.TransactionTiming {
		t.Executed = when
		return t
	})

	if !updated {
		tc.log.Debug().
			Str("transaction_id", txID.String()).
			Msg("failed to update TransactionExecuted metric")
		return
	}

	tc.trackTTE(t, tc.logTimeToExecuted)
	tc.trackTTFE(t, tc.logTimeToFinalizedExecuted)

	// remove transaction timing from mempool if finalized and executed
	if !t.Finalized.IsZero() && !t.Executed.IsZero() {
		tc.transactionTimings.Remove(txID)
	}
}

func (tc *TransactionCollector) trackTTF(t *flow.TransactionTiming, log bool) {
	if t.Received.IsZero() || t.Finalized.IsZero() {
		return
	}

	duration := t.Finalized.Sub(t.Received).Seconds()

	tc.timeToFinalized.Observe(duration)

	if log {
		tc.log.Info().Str("transaction_id", t.TransactionID.String()).Float64("duration", duration).
			Msg("transaction time to finalized")
	}
}

func (tc *TransactionCollector) trackTTE(t *flow.TransactionTiming, log bool) {
	if t.Received.IsZero() || t.Executed.IsZero() {
		return
	}

	duration := t.Executed.Sub(t.Received).Seconds()

	tc.timeToExecuted.Observe(duration)

	if log {
		tc.log.Info().Str("transaction_id", t.TransactionID.String()).Float64("duration", duration).
			Msg("transaction time to executed")
	}
}

func (tc *TransactionCollector) trackTTFE(t *flow.TransactionTiming, log bool) {
	if t.Received.IsZero() || t.Finalized.IsZero() || t.Executed.IsZero() {
		return
	}

	duration := t.Finalized.Sub(t.Received).Seconds()
	if t.Executed.After(t.Finalized) {
		duration = t.Executed.Sub(t.Received).Seconds()
	}

	tc.timeToFinalizedExecuted.Observe(duration)

	if log {
		tc.log.Info().Str("transaction_id", t.TransactionID.String()).Float64("duration", duration).
			Msg("transaction time to finalized and executed")
	}
}

func (tc *TransactionCollector) TransactionSubmissionFailed() {
	tc.transactionSubmission.WithLabelValues("failed").Inc()
}

func (tc *TransactionCollector) TransactionExpired(txID flow.Identifier) {
	_, exist := tc.transactionTimings.ByID(txID)

	if !exist {
		// likely previously removed, either executed or expired
		return
	}
	tc.transactionSubmission.WithLabelValues("expired").Inc()
	tc.transactionTimings.Remove(txID)
}
