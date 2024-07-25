package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/onflow/flow-go/module"
)

type NamespaceType string

const (
	AccessNamespace     NamespaceType = namespaceAccess
	CollectionNamespace NamespaceType = namespaceCollection
)

// TransactionValidationCollector the metrics for transaction validation functionality
type TransactionValidationCollector struct {
	transactionValidated        prometheus.Counter
	transactionValidationFailed *prometheus.CounterVec
}

// interface check
var _ module.TransactionValidationMetrics = (*TransactionValidationCollector)(nil)

// NewTransactionValidationCollector creates new instance of TransactionValidationCollector
func NewTransactionValidationCollector(namespace NamespaceType) *TransactionValidationCollector {
	return &TransactionValidationCollector{
		transactionValidated: promauto.NewCounter(prometheus.CounterOpts{
			Name:      "transaction_validation_succeeded",
			Namespace: string(namespace),
			Subsystem: subsystemTransactionValidation,
			Help:      "counter for the validated transactions",
		}),
		transactionValidationFailed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "transaction_validation_failed",
			Namespace: string(namespace),
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
