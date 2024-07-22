package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// TransactionValidationCollector TODO
type TransactionValidationCollector struct {
	transactionValidated        *prometheus.CounterVec
	transactionValidationFailed *prometheus.CounterVec
}

// interface check
var _ module.TransactionValidationMetrics = (*TransactionValidationCollector)(nil)

func NewTransactionValidationCollector() *TransactionValidationCollector {
	return &TransactionValidationCollector{
		transactionValidated: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_validated_total",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the validated transactions",
		}, []string{"txID"}),
		transactionValidationFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_validation_failed",
			Namespace: namespaceAccess,
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the failed transactions validation",
		}, []string{"txID", "reason"}),
	}
}

// TransactionValidated TODO
func (tc *TransactionValidationCollector) TransactionValidated(txID flow.Identifier) {
	tc.transactionValidated.WithLabelValues(txID.String()).Inc()
}

// TransactionValidationFailed TODO
func (tc *TransactionValidationCollector) TransactionValidationFailed(txID flow.Identifier, reason string) {
	tc.transactionValidationFailed.WithLabelValues(txID.String(), reason).Inc()
}
