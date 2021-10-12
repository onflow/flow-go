package unstaked

import "github.com/onflow/flow-go/module"

type EngineCollector struct {
	metrics module.EngineMetrics
}

func NewUnstakedEngineCollector(metrics module.EngineMetrics) *EngineCollector {
	return &EngineCollector{metrics}
}

func (ec *EngineCollector) MessageSent(engine string, message string) {
	ec.metrics.MessageSent("unstaked_"+engine, message)
}

func (ec *EngineCollector) MessageReceived(engine string, message string) {
	ec.metrics.MessageReceived("unstaked_"+engine, message)
}

func (ec *EngineCollector) MessageHandled(engine string, message string) {
	ec.metrics.MessageHandled("unstaked_"+engine, message)
}
