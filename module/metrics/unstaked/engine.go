package unstaked

import (
	"github.com/onflow/flow-go/module/metrics"
)

type EngineCollector struct {
	collector metrics.EngineCollector
}

func NewUnstakedEngineCollector(collector metrics.EngineCollector) *EngineCollector {
	return &EngineCollector{collector}
}

func (ec *EngineCollector) MessageSent(engine string, message string) {
	ec.collector.MessageSent("unstaked_"+engine, message)
}

func (ec *EngineCollector) MessageReceived(engine string, message string) {
	ec.collector.MessageReceived("unstaked_"+engine, message)
}

func (ec *EngineCollector) MessageHandled(engine string, message string) {
	ec.collector.MessageHandled("unstaked_"+engine, message)
}
