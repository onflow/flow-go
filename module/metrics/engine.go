package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type EngineCollector struct {
	sent     *prometheus.CounterVec
	received *prometheus.CounterVec
}

func NewEngineCollector() *EngineCollector {

	ec := &EngineCollector{

		sent: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "messages_sent_total",
			Namespace: namespaceNetwork,
			Subsystem: subsystemEngine,
			Help:      "the number of messages sent by engines",
		}, []string{EngineLabel, LabelMessage}),

		received: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "messages_received_total",
			Namespace: namespaceNetwork,
			Subsystem: subsystemEngine,
			Help:      "the number of messages received by engines",
		}, []string{EngineLabel, LabelMessage}),
	}

	return ec
}

func (ec *EngineCollector) MessageSent(engine string, message string) {
	ec.sent.With(prometheus.Labels{EngineLabel: engine, LabelMessage: message}).Inc()
}

func (ec *EngineCollector) MessageReceived(engine string, message string) {
	ec.received.With(prometheus.Labels{EngineLabel: engine, LabelMessage: message}).Inc()
}
