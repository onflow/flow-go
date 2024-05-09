package metrics

import (
	"github.com/onflow/flow-go/module"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type EngineCollector struct {
	sent            *prometheus.CounterVec
	received        *prometheus.CounterVec
	handled         *prometheus.CounterVec
	inboundDropped  *prometheus.CounterVec
	outboundDropped *prometheus.CounterVec
}

var _ module.EngineMetrics = (*EngineCollector)(nil)

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

		handled: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "messages_handled_total",
			Namespace: namespaceNetwork,
			Subsystem: subsystemEngine,
			Help:      "the number of messages handled by engines",
		}, []string{EngineLabel, LabelMessage}),

		inboundDropped: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "inbound_messages_dropped_total",
			Namespace: namespaceNetwork,
			Subsystem: subsystemEngine,
			Help:      "the number of inbound messages dropped by engines",
		}, []string{EngineLabel, LabelMessage}),

		outboundDropped: promauto.NewCounterVec(prometheus.CounterOpts{
			Name:      "outbound_messages_dropped_total",
			Namespace: namespaceNetwork,
			Subsystem: subsystemEngine,
			Help:      "the number of outbound messages dropped by engines",
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

func (ec *EngineCollector) MessageHandled(engine string, message string) {
	ec.handled.With(prometheus.Labels{EngineLabel: engine, LabelMessage: message}).Inc()
}

func (ec *EngineCollector) InboundMessageDropped(engine string, message string) {
	ec.inboundDropped.With(prometheus.Labels{EngineLabel: engine, LabelMessage: message}).Inc()
}

func (ec *EngineCollector) OutboundMessageDropped(engine string, message string) {
	ec.outboundDropped.With(prometheus.Labels{EngineLabel: engine, LabelMessage: message}).Inc()
}
