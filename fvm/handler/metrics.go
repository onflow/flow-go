package handler

import (
	"time"

	"github.com/onflow/cadence/runtime/common"
)

// MetricsReporter captures and reports metrics to back to the execution environment
// it is a setup passed to the context.
//
// TODO expand this to more metrics
type MetricsReporter interface {
	TransactionParsed(time.Duration)
	TransactionChecked(time.Duration)
	TransactionInterpreted(time.Duration)
}

// A MetricsHandler accumulates performance metrics reported by the Cadence runtime.
type MetricsHandler struct {
	Reporter                 MetricsReporter
	TimeSpentOnParsing       time.Duration
	TimeSpentOnChecking      time.Duration
	TimeSpentOnInterpreting  time.Duration
	TimeSpentOnValueEncoding time.Duration
	TimeSpentOnValueDecoding time.Duration
}

// NewMetricsHandler constructs a MetricsHandler
func NewMetricsHandler(reporter MetricsReporter) *MetricsHandler {
	return &MetricsHandler{Reporter: reporter}
}

// ProgramParsed captures time spent on parsing a code at specific location
func (m *MetricsHandler) ProgramParsed(location common.Location, duration time.Duration) {
	// These checks prevent re-reporting durations, the metrics collection is a bit counter-intuitive:
	// The three functions (parsing, checking, interpretation) are not called in sequence, but in some cases as part of each other.
	// We basically only measure the durations reported for the entry-point (the transaction), and not for child locations,
	// because they might be already part of the duration for the entry-point.
	if _, ok := location.(common.TransactionLocation); ok {
		m.TimeSpentOnParsing = duration
		m.Reporter.TransactionParsed(duration)
	}
}

// ProgramChecked captures time spent on checking a code at specific location
func (m *MetricsHandler) ProgramChecked(location common.Location, duration time.Duration) {
	// see the comment for ProgramParsed
	if _, ok := location.(common.TransactionLocation); ok {
		m.TimeSpentOnChecking = duration
		m.Reporter.TransactionChecked(duration)
	}
}

// ProgramInterpreted captures time spent on interpreting a code at specific location
func (m *MetricsHandler) ProgramInterpreted(location common.Location, duration time.Duration) {
	// see the comment for ProgramInterpreted
	if _, ok := location.(common.TransactionLocation); ok {
		m.TimeSpentOnInterpreting = duration
		m.Reporter.TransactionInterpreted(duration)
	}
}

// ValueEncoded accumulates time spend on runtime value encoding
func (m *MetricsHandler) ValueEncoded(duration time.Duration) {
	m.TimeSpentOnValueEncoding += duration
}

// ValueDecoded accumulates time spend on runtime value decoding
func (m *MetricsHandler) ValueDecoded(duration time.Duration) {
	m.TimeSpentOnValueDecoding += duration
}

// NoopMetricsReporter is a MetricReporter that does nothing.
type NoopMetricsReporter struct{}

// TransactionParsed is a noop
func (NoopMetricsReporter) TransactionParsed(time.Duration) {}

// TransactionChecked is a noop
func (NoopMetricsReporter) TransactionChecked(time.Duration) {}

// TransactionInterpreted is a noop
func (NoopMetricsReporter) TransactionInterpreted(time.Duration) {}
