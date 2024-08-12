package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type NamespaceType string

// TransactionValidationCollector the metrics for transaction validation functionality
type TransactionValidationCollector struct {
	transactionValidated         prometheus.Counter
	transactionValidationSkipped prometheus.Counter
	transactionValidationFailed  *prometheus.CounterVec
}

// interface check
var _ module.TransactionValidationMetrics = (*TransactionValidationCollector)(nil)

// NewTransactionValidationCollector creates new instance of TransactionValidationCollector
func NewTransactionValidationCollector() *TransactionValidationCollector {
	return &TransactionValidationCollector{
		transactionValidated: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transaction_validation_successes_total",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the validated transactions",
		}),
		transactionValidationSkipped: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transaction_validation_skipped",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the skipped transaction validations",
		}),
		transactionValidationFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_validation_failed",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the failed transactions validation",
		}, []string{"reason"}),
	}
}

// TransactionValidated tracks number of successfully validated transactions
func (tc *TransactionValidationCollector) TransactionValidated() {
	tc.transactionValidated.Inc()
}

// TransactionValidationFailed tracks number of validation failed transactions with reason
func (tc *TransactionValidationCollector) TransactionValidationFailed(reason string) {
	tc.transactionValidationFailed.WithLabelValues(reason).Inc()
}

// TransactionValidationSkipped tracks number of skipped transaction validations
func (tc *TransactionValidationCollector) TransactionValidationSkipped() {
	tc.transactionValidationSkipped.Inc()
}
