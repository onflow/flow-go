package virtualmachine

import (
	"time"

	"github.com/onflow/cadence/runtime/ast"
)

// A MetricsCollector accumulates performance metrics reported by the Cadence runtime.
//
// A single collector instance will sum all reported values. For example, the "parsed" field will be
// incremented each time a program is parsed.
type MetricsCollector struct {
	parsed      time.Duration
	checked     time.Duration
	interpreted time.Duration
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{}
}

func (m *MetricsCollector) Parsed() time.Duration      { return m.parsed }
func (m *MetricsCollector) Checked() time.Duration     { return m.checked }
func (m *MetricsCollector) Interpreted() time.Duration { return m.interpreted }

type metricsCollector struct {
	*MetricsCollector
}

func (m metricsCollector) ProgramParsed(location ast.Location, duration time.Duration) {
	if m.MetricsCollector == nil {
		return
	}
	m.parsed += duration
}

func (m metricsCollector) ProgramChecked(location ast.Location, duration time.Duration) {
	if m.MetricsCollector == nil {
		return
	}
	m.checked += duration
}

func (m metricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {
	if m.MetricsCollector == nil {
		return
	}
	m.interpreted += duration
}

func (m metricsCollector) ValueEncoded(duration time.Duration) {}
func (m metricsCollector) ValueDecoded(duration time.Duration) {}

type emptyMetricsCollector struct{}

func (m emptyMetricsCollector) ProgramParsed(location ast.Location, duration time.Duration)      {}
func (m emptyMetricsCollector) ProgramChecked(location ast.Location, duration time.Duration)     {}
func (m emptyMetricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {}
func (m emptyMetricsCollector) ValueEncoded(duration time.Duration)                              {}
func (m emptyMetricsCollector) ValueDecoded(duration time.Duration)                              {}
