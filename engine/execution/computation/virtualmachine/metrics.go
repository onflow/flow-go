package virtualmachine

import (
	"time"

	"github.com/onflow/cadence/runtime/ast"
)

type RuntimeMetrics struct {
	parsed      time.Duration
	checked     time.Duration
	interpreted time.Duration
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{}
}

func (m *RuntimeMetrics) Parsed() time.Duration      { return m.parsed }
func (m *RuntimeMetrics) Checked() time.Duration     { return m.checked }
func (m *RuntimeMetrics) Interpreted() time.Duration { return m.interpreted }

type runtimeMetricsCollector struct {
	*RuntimeMetrics
}

func (m runtimeMetricsCollector) ProgramParsed(location ast.Location, duration time.Duration) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.parsed += duration
}

func (m runtimeMetricsCollector) ProgramChecked(location ast.Location, duration time.Duration) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.checked += duration
}

func (m runtimeMetricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.interpreted += duration
}

func (m runtimeMetricsCollector) ValueEncoded(duration time.Duration) {}
func (m runtimeMetricsCollector) ValueDecoded(duration time.Duration) {}

type emptyMetricsCollector struct{}

func (m emptyMetricsCollector) ProgramParsed(location ast.Location, duration time.Duration)      {}
func (m emptyMetricsCollector) ProgramChecked(location ast.Location, duration time.Duration)     {}
func (m emptyMetricsCollector) ProgramInterpreted(location ast.Location, duration time.Duration) {}
func (m emptyMetricsCollector) ValueEncoded(duration time.Duration)                              {}
func (m emptyMetricsCollector) ValueDecoded(duration time.Duration)                              {}
