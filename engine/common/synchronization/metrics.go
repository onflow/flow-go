package synchronization

import (
	"github.com/onflow/flow-go/module"
)

// SyncEngineMetrics is a helper interface which is built around module.EngineMetrics.
// It's useful for RequestHandlerEngine since it can be used to report metrics for different engine names.
type SyncEngineMetrics interface {
	MessageSent(message string)
	MessageReceived(message string)
	MessageHandled(messages string)
}

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
