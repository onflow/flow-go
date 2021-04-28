package handler

import (
	"time"

	"github.com/onflow/cadence/runtime/common"
)

// MetricsReporter captures and reports execution metrics
type MetricsReporter interface {
	TransactionParsed(time.Duration)
	TransactionChecked(time.Duration)
	TransactionInterpreted(time.Duration)
}

// A MetricsHandler accumulates performance metrics reported by the Cadence runtime.
//
// A single collector instance will sum all reported values. For example, the "parsed" field will be
// incremented each time a program is parsed.
type MetricsHandler struct {
	Reporter                 MetricsReporter
	TimeSpentOnParsing       time.Duration
	TimeSpentOnChecking      time.Duration
	TimeSpentOnInterpreting  time.Duration
	TimeSpentOnValueEncoding time.Duration
	TimeSpentOnValueDecoding time.Duration
}

func NewMetricsHandler(reporter MetricsReporter) *MetricsHandler {
	return &MetricsHandler{Reporter: reporter}
}

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

func (m *MetricsHandler) ProgramChecked(location common.Location, duration time.Duration) {
	// see the comment for ProgramParsed
	if _, ok := location.(common.TransactionLocation); ok {
		m.TimeSpentOnChecking = duration
		m.Reporter.TransactionChecked(duration)
	}
}

func (m *MetricsHandler) ProgramInterpreted(location common.Location, duration time.Duration) {
	// see the comment for ProgramInterpreted
	if _, ok := location.(common.TransactionLocation); ok {
		m.TimeSpentOnInterpreting = duration
		m.Reporter.TransactionInterpreted(duration)
	}
}

func (m *MetricsHandler) ValueEncoded(duration time.Duration) {
	m.TimeSpentOnValueEncoding += duration
}

func (m *MetricsHandler) ValueDecoded(duration time.Duration) {
	m.TimeSpentOnValueDecoding += duration
}

type NoopMetricsReporter struct{}

func (NoopMetricsReporter) TransactionParsed(time.Duration)      {}
func (NoopMetricsReporter) TransactionChecked(time.Duration)     {}
func (NoopMetricsReporter) TransactionInterpreted(time.Duration) {}
