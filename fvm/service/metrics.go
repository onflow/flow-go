package service

import (
	"time"

	"github.com/onflow/cadence/runtime/ast"
)

// A MetricsCollector accumulates performance metrics reported by the Cadence runtime.
//
// A single collector instance will sum all reported values. For example, the "parsed" field will be
// incremented each time a program is parsed.
type DefaultMetricsCollector struct {
	parsed      time.Duration
	checked     time.Duration
	interpreted time.Duration
}

// NewMetricsCollectors returns a new runtime metrics collector.
func NewMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{}
}

func (m *DefaultMetricsCollector) Parsed() time.Duration      { return m.parsed }
func (m *DefaultMetricsCollector) Checked() time.Duration     { return m.checked }
func (m *DefaultMetricsCollector) Interpreted() time.Duration { return m.interpreted }
func (m *DefaultMetricsCollector) ProgramParsed(location ast.Location, duration time.Duration) {
	m.parsed += duration
}
func (m *DefaultMetricsCollector) ProgramChecked(location ast.Location, duration time.Duration) {
	m.checked += duration
}
func (m *DefaultMetricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {
	m.interpreted += duration
}
func (m *DefaultMetricsCollector) ValueEncoded(duration time.Duration) {}
func (m *DefaultMetricsCollector) ValueDecoded(duration time.Duration) {}

type NoopMetricsCollector struct{}

func (m *NoopMetricsCollector) Parsed() time.Duration                                            { return 0 }
func (m *NoopMetricsCollector) Checked() time.Duration                                           { return 0 }
func (m *NoopMetricsCollector) Interpreted() time.Duration                                       { return 0 }
func (m *NoopMetricsCollector) ProgramParsed(location ast.Location, duration time.Duration)      {}
func (m *NoopMetricsCollector) ProgramChecked(location ast.Location, duration time.Duration)     {}
func (m *NoopMetricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {}
func (m *NoopMetricsCollector) ValueEncoded(duration time.Duration)                              {}
func (m *NoopMetricsCollector) ValueDecoded(duration time.Duration)                              {}
