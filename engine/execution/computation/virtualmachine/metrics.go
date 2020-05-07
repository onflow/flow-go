package virtualmachine

import (
	"time"
)

type RuntimeMetrics struct {
	parsed      time.Duration
	checked     time.Duration
	interpreted time.Duration
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{}
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

func (m *RuntimeMetrics) Parsed() time.Duration      { return m.parsed }
func (m *RuntimeMetrics) Checked() time.Duration     { return m.checked }
func (m *RuntimeMetrics) Interpreted() time.Duration { return m.interpreted }

type runtimeMetricsCollector struct {
	*RuntimeMetrics
}

func (m runtimeMetricsCollector) ProgramParsed(elapsed time.Duration) {
	m.parsed = elapsed
}

func (m runtimeMetricsCollector) ProgramChecked(elapsed time.Duration) {
	m.checked = elapsed
}

func (m runtimeMetricsCollector) ProgramInterpreted(elapsed time.Duration) {
	m.interpreted = elapsed
}

func (m runtimeMetricsCollector) ValueEncoded(elapsed time.Duration) {}
func (m runtimeMetricsCollector) ValueDecoded(elapsed time.Duration) {}

type emptyMetricsCollector struct{}

func (m emptyMetricsCollector) ProgramParsed(elapsed time.Duration) {}

func (m emptyMetricsCollector) ProgramChecked(elapsed time.Duration) {}

func (m emptyMetricsCollector) ProgramInterpreted(elapsed time.Duration) {}

func (m emptyMetricsCollector) ValueEncoded(elapsed time.Duration) {}
func (m emptyMetricsCollector) ValueDecoded(elapsed time.Duration) {}
