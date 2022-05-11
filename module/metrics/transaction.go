package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
)

type TransactionCollector struct {
	transactionTimings         mempool.TransactionTimings
	log                        zerolog.Logger
	logTimeToFinalized         bool
	logTimeToExecuted          bool
	logTimeToFinalizedExecuted bool
	timeToFinalized            prometheus.Summary
	timeToExecuted             prometheus.Summary
	timeToFinalizedExecuted    prometheus.Summary
	transactionSubmission      *prometheus.CounterVec
	executeScriptDuration      *prometheus.CounterVec
}

func NewTransactionCollector(transactionTimings mempool.TransactionTimings, log zerolog.Logger,
	logTimeToFinalized bool, logTimeToExecuted bool, logTimeToFinalizedExecuted bool) *TransactionCollector {

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
		executeScriptDuration: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "execute_script_rtt_duration",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionSubmission,
			Help:      "counter for the round trip time for executing a script",
		}, []string{"script_size"}),
	}

	return tc
}

func (tc *TransactionCollector) ExecuteScriptRTT(dur time.Duration, size int) {
	sizeKb := size / 1024
	sizeLabel := "100+kb" //"1kb,10kb,100kb, 100+kb" -> [0,1] [2,10] [11,100] [100, +inf]

	if sizeKb <= 1 {
		sizeLabel = "1kb"
	} else if sizeKb <= 10 {
		sizeLabel = "10kb"
	} else if sizeKb <= 100 {
		sizeLabel = "100kb"
	}

	// record the script
	tc.executeScriptDuration.With(prometheus.Labels{
		"script_size": sizeLabel,
	}).Add(float64(dur.Milliseconds()))
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
		tc.transactionTimings.Rem(txID)
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
		tc.transactionTimings.Rem(txID)
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
	tc.transactionTimings.Rem(txID)
}
