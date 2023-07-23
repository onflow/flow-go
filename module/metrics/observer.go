package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc/codes"
)

type ObserverMetrics interface {
	RecordRPC(handler, rpc string, code codes.Code)
}

type ObserverCollector struct {
	rpcs *prometheus.CounterVec
}

var _ ObserverMetrics = (*ObserverCollector)(nil)

func NewObserverCollector() *ObserverCollector {
	return &ObserverCollector{
		rpcs: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespaceObserver,
			Subsystem: subsystemObserverGRPC,
			Name:      "handler_grpc_counter",
			Help:      "tracking error/success rate of each rpc for the observer service",
		}, []string{"handler", "grpc_method", "grpc_code"}),
	}
}

func (oc *ObserverCollector) RecordRPC(handler, rpc string, code codes.Code) {
	oc.rpcs.With(prometheus.Labels{
		"handler":     handler,
		"grpc_method": rpc,
		"grpc_code":   code.String(),
	}).Inc()
}
