package virtualmachine

import (
	"time"
)

type RuntimeMetrics struct {
	parsed      TimeRange
	checked     TimeRange
	interpreted TimeRange
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{}
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (m *RuntimeMetrics) Parsed() TimeRange      { return m.parsed }
func (m *RuntimeMetrics) Checked() TimeRange     { return m.checked }
func (m *RuntimeMetrics) Interpreted() TimeRange { return m.interpreted }

type runtimeMetricsCollector struct {
	*RuntimeMetrics
}

func (m runtimeMetricsCollector) ProgramParsed(start, end time.Time) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.parsed = TimeRange{Start: start, End: end}
}

func (m runtimeMetricsCollector) ProgramChecked(start, end time.Time) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.checked = TimeRange{Start: start, End: end}
}

func (m runtimeMetricsCollector) ProgramInterpreted(start, end time.Time) {
	if m.RuntimeMetrics == nil {
		return
	}
	m.interpreted = TimeRange{Start: start, End: end}
}

func (m runtimeMetricsCollector) ValueEncoded(start, end time.Time) {}
func (m runtimeMetricsCollector) ValueDecoded(start, end time.Time) {}

type emptyMetricsCollector struct{}

func (m emptyMetricsCollector) ProgramParsed(start, end time.Time) {}

func (m emptyMetricsCollector) ProgramChecked(start, end time.Time) {}

func (m emptyMetricsCollector) ProgramInterpreted(start, end time.Time) {}

func (m emptyMetricsCollector) ValueEncoded(start, end time.Time) {}
func (m emptyMetricsCollector) ValueDecoded(start, end time.Time) {}
