package synchronization

import (
	"github.com/onflow/flow-go/module"
)

// SyncEngineMetricsImpl implements SyncEngineMetrics proxying all calls to module.EngineMetrics with
// preset engine name at construction time.
type SyncEngineMetricsImpl struct {
	engineName string
	metrics    module.EngineMetrics
}

func NewSyncEngineMetrics(engineName string, metrics module.EngineMetrics) *SyncEngineMetricsImpl {
	return &SyncEngineMetricsImpl{
		engineName: engineName,
		metrics:    metrics,
	}
}

func (r *SyncEngineMetricsImpl) MessageSent(message string) {
	r.metrics.MessageSent(r.engineName, message)
}

func (r *SyncEngineMetricsImpl) MessageReceived(message string) {
	r.metrics.MessageReceived(r.engineName, message)
}

func (r *SyncEngineMetricsImpl) MessageHandled(message string) {
	r.metrics.MessageHandled(r.engineName, message)
}

// NoopSyncEngineMetrics is a dummy implementation which doesn't report any metrics
type NoopSyncEngineMetrics struct {
}

func NewNoopSyncEngineMetrics() *NoopSyncEngineMetrics {
	return &NoopSyncEngineMetrics{}
}

func (n *NoopSyncEngineMetrics) MessageSent(message string) {

}

func (n *NoopSyncEngineMetrics) MessageReceived(message string) {

}

func (n *NoopSyncEngineMetrics) MessageHandled(messages string) {

}
