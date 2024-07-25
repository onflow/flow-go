package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

// TransactionValidationCollector TODO
type TransactionValidationCollector struct {
	transactionValidated        prometheus.Counter
	transactionValidationFailed *prometheus.CounterVec
}

// interface check
var _ module.TransactionValidationMetrics = (*TransactionValidationCollector)(nil)

func NewTransactionValidationCollector() *TransactionValidationCollector {
	return &TransactionValidationCollector{
		transactionValidated: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transaction_validation_succeeded",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the validated transactions",
		}),
		transactionValidationFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_validation_failed",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the failed transactions validation",
		}, []string{"reason"}),
	}
}

// TransactionValidated TODO
func (tc *TransactionValidationCollector) TransactionValidated() {
	tc.transactionValidated.Inc()
}

// TransactionValidationFailed TODO
func (tc *TransactionValidationCollector) TransactionValidationFailed(reason string) {
	tc.transactionValidationFailed.WithLabelValues(reason).Inc()
}
