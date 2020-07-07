package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
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
				0.5:  0.01,
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
				0.5:  0.01,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
		timeToFinalizedExecuted: promauto.NewSummary(prometheus.SummaryOpts{
			Name:      "time_to_executed_finalized_seconds",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionTiming,
			Help: "the duration of how long it took between the transaction was received until it was both " +
				"finalized and executed",
			Objectives: map[float64]float64{
				0.01: 0.001,
				0.5:  0.01,
				0.99: 0.001,
			},
			MaxAge:     10 * time.Minute,
			AgeBuckets: 5,
			BufCap:     500,
		}),
	}

	return tc
}

func (tc *TransactionCollector) TransactionReceived(txID flow.Identifier, when time.Time) {
	// we don't need to check whether the transaction timing already exists, it will not be overwritten by the mempool
	tc.transactionTimings.Add(&flow.TransactionTiming{TransactionID: txID, Received: when})
}

func (tc *TransactionCollector) TransactionFinalized(txID flow.Identifier, when time.Time) {
	t, updated := tc.transactionTimings.Adjust(txID, func(t *flow.TransactionTiming) *flow.TransactionTiming {
		t.Finalized = when
		return t
	})

	if !updated {
		return
	}

	if tc.logTimeToFinalized {
		tc.logTTF(t)
	}

	if tc.logTimeToFinalizedExecuted {
		tc.logTTEF(t)
	}

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
		return
	}

	if tc.logTimeToExecuted {
		tc.logTTE(t)
	}

	if tc.logTimeToFinalizedExecuted {
		tc.logTTEF(t)
	}

	// remove transaction timing from mempool if finalized and executed
	if !t.Finalized.IsZero() && !t.Executed.IsZero() {
		tc.transactionTimings.Rem(txID)
	}
}

func (tc *TransactionCollector) logTTF(t *flow.TransactionTiming) {
	if t.Received.IsZero() || t.Finalized.IsZero() {
		return
	}

	tc.log.Info().Str("transaction_id", t.TransactionID.String()).Int64("duration", int64(t.Finalized.Sub(t.Received))).
		Msg("transaction time to finalized")
}

func (tc *TransactionCollector) logTTE(t *flow.TransactionTiming) {
	if t.Received.IsZero() || t.Executed.IsZero() {
		return
	}

	tc.log.Info().Str("transaction_id", t.TransactionID.String()).Int64("duration", int64(t.Executed.Sub(t.Received))).
		Msg("transaction time to executed")
}

func (tc *TransactionCollector) logTTEF(t *flow.TransactionTiming) {
	if t.Received.IsZero() || t.Finalized.IsZero() || t.Executed.IsZero() {
		return
	}

	finalizedAndExecuted := t.Finalized.Sub(t.Received)
	if t.Executed.After(t.Finalized) {
		finalizedAndExecuted = t.Executed.Sub(t.Received)
	}
	tc.log.Info().Str("transaction_id", t.TransactionID.String()).Int64("duration", int64(finalizedAndExecuted)).
		Msg("transaction time to finalized and executed")
}
