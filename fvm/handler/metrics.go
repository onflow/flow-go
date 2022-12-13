package handler

import (
	"time"
)

// MetricsReporter captures and reports metrics to back to the execution environment
// it is a setup passed to the context.
type MetricsReporter interface {
	RuntimeTransactionParsed(time.Duration)
	RuntimeTransactionChecked(time.Duration)
	RuntimeTransactionInterpreted(time.Duration)
	RuntimeSetNumberOfAccounts(count uint64)
}

// NoopMetricsReporter is a MetricReporter that does nothing.
type NoopMetricsReporter struct{}

// RuntimeTransactionParsed is a noop
func (NoopMetricsReporter) RuntimeTransactionParsed(time.Duration) {}

// RuntimeTransactionChecked is a noop
func (NoopMetricsReporter) RuntimeTransactionChecked(time.Duration) {}

// RuntimeTransactionInterpreted is a noop
func (NoopMetricsReporter) RuntimeTransactionInterpreted(time.Duration) {}

// RuntimeSetNumberOfAccounts is a noop
func (NoopMetricsReporter) RuntimeSetNumberOfAccounts(count uint64) {}
